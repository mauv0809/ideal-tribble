package charts

import (
	"fmt"
	"html/template"
	"strings"
)

// LineChart renders a line chart with optional area fill.
type LineChart struct {
	config   ChartConfig
	series   []Series
	showArea bool
	smooth   bool // Use bezier curves for smooth lines
}

// NewLineChart creates a new line chart.
func NewLineChart() *LineChart {
	return &LineChart{
		config: DefaultConfig(),
		series: make([]Series, 0),
	}
}

// SetDimensions sets the chart dimensions.
func (c *LineChart) SetDimensions(width, height int) Chart {
	c.config.Width = width
	c.config.Height = height
	return c
}

// SetTheme sets the chart theme.
func (c *LineChart) SetTheme(theme Theme) Chart {
	c.config.Theme = theme
	return c
}

// SetTitle sets the chart title.
func (c *LineChart) SetTitle(title string) Chart {
	c.config.Title = title
	return c
}

// SetConfig allows setting the full config.
func (c *LineChart) SetConfig(config ChartConfig) *LineChart {
	c.config = config
	return c
}

// AddSeries adds a data series to the chart.
func (c *LineChart) AddSeries(series Series) *LineChart {
	c.series = append(c.series, series)
	return c
}

// AddDataPoints is a convenience method to add a single series.
func (c *LineChart) AddDataPoints(name string, points []DataPoint) *LineChart {
	c.series = append(c.series, Series{
		Name:   name,
		Points: points,
	})
	return c
}

// WithArea enables area fill under the line.
func (c *LineChart) WithArea(enabled bool) *LineChart {
	c.showArea = enabled
	return c
}

// WithSmooth enables smooth bezier curves.
func (c *LineChart) WithSmooth(enabled bool) *LineChart {
	c.smooth = enabled
	return c
}

// Render generates the SVG.
func (c *LineChart) Render() (template.HTML, error) {
	if len(c.series) == 0 {
		return "", fmt.Errorf("no data series provided")
	}

	svg := NewSVGBuilder(c.config.Width, c.config.Height)
	theme := c.config.Theme

	// Calculate effective bottom padding (add space for legend if needed)
	bottomPadding := c.config.PaddingBottom
	if c.config.ShowLegend && len(c.series) > 1 {
		bottomPadding += 25 // Extra space for legend below chart
	}

	// Calculate chart area
	chartX := float64(c.config.PaddingLeft)
	chartY := float64(c.config.PaddingTop)
	chartWidth := float64(c.config.Width - c.config.PaddingLeft - c.config.PaddingRight)
	chartHeight := float64(c.config.Height - c.config.PaddingTop - bottomPadding)

	// Find data bounds
	var minY, maxY float64 = 0, 0
	var maxPoints int = 0
	first := true

	for _, s := range c.series {
		if len(s.Points) > maxPoints {
			maxPoints = len(s.Points)
		}
		for _, p := range s.Points {
			if first {
				minY = p.Value
				maxY = p.Value
				first = false
			} else {
				if p.Value < minY {
					minY = p.Value
				}
				if p.Value > maxY {
					maxY = p.Value
				}
			}
		}
	}

	// Calculate nice scale
	minY, maxY, tickInterval := CalculateNiceScale(minY, maxY, 5)

	// Add gradient definitions for area fills
	if c.showArea {
		for i := range c.series {
			color := c.getSeriesColor(i)
			gradientID := fmt.Sprintf("area-gradient-%d", i)
			svg.AddDef(fmt.Sprintf(
				`<linearGradient id="%s" x1="0%%" y1="0%%" x2="0%%" y2="100%%">
					<stop offset="0%%" style="stop-color:%s;stop-opacity:0.3"/>
					<stop offset="100%%" style="stop-color:%s;stop-opacity:0.05"/>
				</linearGradient>`,
				gradientID, color, color,
			))
		}
	}

	// Draw background (optional)
	if theme.ChartArea != "transparent" {
		svg.Rect(chartX, chartY, chartWidth, chartHeight, map[string]string{
			"fill": theme.ChartArea,
		})
	}

	// Draw grid lines
	if c.config.ShowGrid {
		// Horizontal grid lines
		for y := minY; y <= maxY; y += tickInterval {
			yPos := chartY + chartHeight - ((y-minY)/(maxY-minY))*chartHeight
			svg.Line(chartX, yPos, chartX+chartWidth, yPos, map[string]string{
				"stroke":       theme.GridColor,
				"stroke-width": "1",
			})
		}
	}

	// Draw axes
	if c.config.ShowYAxis {
		// Y-axis line
		svg.Line(chartX, chartY, chartX, chartY+chartHeight, map[string]string{
			"stroke":       theme.AxisColor,
			"stroke-width": "1",
		})

		// Y-axis labels
		for y := minY; y <= maxY; y += tickInterval {
			yPos := chartY + chartHeight - ((y-minY)/(maxY-minY))*chartHeight
			svg.Text(chartX-8, yPos+4, FormatTickValue(y), map[string]string{
				"fill":        theme.TextMuted,
				"font-size":   "11",
				"text-anchor": "end",
				"font-family": "system-ui, sans-serif",
			})
		}
	}

	if c.config.ShowXAxis {
		// X-axis line
		svg.Line(chartX, chartY+chartHeight, chartX+chartWidth, chartY+chartHeight, map[string]string{
			"stroke":       theme.AxisColor,
			"stroke-width": "1",
		})

		// X-axis labels (based on maxPoints across all series)
		if maxPoints > 0 {
			step := 1
			if maxPoints > 10 {
				step = maxPoints / 5 // Show ~5 labels max
			}

			for i := 0; i < maxPoints; i += step {
				xPos := chartX + (float64(i)/float64(maxPoints-1))*chartWidth
				label := fmt.Sprintf("%d", i+1) // Numeric labels
				// Try to use first series label if available
				if len(c.series) > 0 && i < len(c.series[0].Points) {
					label = c.series[0].Points[i].Label
				}
				svg.Text(xPos, chartY+chartHeight+16, label, map[string]string{
					"fill":        theme.TextMuted,
					"font-size":   "10",
					"text-anchor": "middle",
					"font-family": "system-ui, sans-serif",
				})
			}
			// Always show the last label
			if maxPoints > 1 {
				xPos := chartX + chartWidth
				label := fmt.Sprintf("%d", maxPoints)
				svg.Text(xPos, chartY+chartHeight+16, label, map[string]string{
					"fill":        theme.TextMuted,
					"font-size":   "10",
					"text-anchor": "middle",
					"font-family": "system-ui, sans-serif",
				})
			}
		}
	}

	// Draw series
	for seriesIdx, s := range c.series {
		if len(s.Points) < 2 {
			continue
		}

		color := c.getSeriesColor(seriesIdx)
		points := make([]struct{ x, y float64 }, len(s.Points))

		// Calculate point positions
		for i, p := range s.Points {
			points[i].x = chartX + (float64(i)/float64(len(s.Points)-1))*chartWidth
			points[i].y = chartY + chartHeight - ((p.Value-minY)/(maxY-minY))*chartHeight
		}

		// Build path
		var pathD string
		if c.smooth && len(points) > 2 {
			pathD = c.buildSmoothPath(points)
		} else {
			pathD = c.buildLinearPath(points)
		}

		// Draw area fill first (behind line)
		if c.showArea {
			areaPath := pathD
			// Close the path along the bottom
			areaPath += fmt.Sprintf(" L %.2f %.2f L %.2f %.2f Z",
				points[len(points)-1].x, chartY+chartHeight,
				points[0].x, chartY+chartHeight,
			)
			svg.Path(areaPath, map[string]string{
				"fill": fmt.Sprintf("url(#area-gradient-%d)", seriesIdx),
			})
		}

		// Draw line
		svg.Path(pathD, map[string]string{
			"fill":         "none",
			"stroke":       color,
			"stroke-width": "2",
			"stroke-linecap": "round",
			"stroke-linejoin": "round",
		})

		// Draw data points (circles)
		for i, pt := range points {
			attrs := map[string]string{
				"fill":   color,
				"stroke": theme.Background,
				"stroke-width": "2",
			}

			// Add data attributes for interactivity
			if c.config.EnableDataAttributes {
				attrs["data-series"] = s.Name
				attrs["data-index"] = fmt.Sprintf("%d", i)
				attrs["data-value"] = fmt.Sprintf("%.2f", s.Points[i].Value)
				attrs["data-label"] = s.Points[i].Label
				attrs["class"] = "chart-point"

				// Add custom metadata
				for k, v := range s.Points[i].Meta {
					attrs["data-"+k] = v
				}
			}

			svg.Circle(pt.x, pt.y, 4, attrs)
		}
	}

	// Draw title
	if c.config.Title != "" {
		svg.Text(float64(c.config.Width)/2, 20, c.config.Title, map[string]string{
			"fill":        theme.TextColor,
			"font-size":   "14",
			"font-weight": "600",
			"text-anchor": "middle",
			"font-family": "system-ui, sans-serif",
		})
	}

	// Draw legend below the chart
	if c.config.ShowLegend && len(c.series) > 1 {
		// Position legend below the X-axis, horizontally centered
		legendY := chartY + chartHeight + 28 // Below X-axis labels

		// Calculate total legend width for centering
		itemWidth := 70.0 // Approximate width per legend item
		totalWidth := float64(len(c.series)) * itemWidth
		legendX := chartX + (chartWidth-totalWidth)/2
		if legendX < chartX {
			legendX = chartX
		}

		for i, s := range c.series {
			color := c.getSeriesColor(i)
			xOffset := float64(i) * itemWidth

			// Color marker (small line segment)
			svg.Line(legendX+xOffset, legendY, legendX+xOffset+16, legendY, map[string]string{
				"stroke":       color,
				"stroke-width": "3",
			})

			// Label
			svg.Text(legendX+xOffset+20, legendY+4, s.Name, map[string]string{
				"fill":        theme.TextColor,
				"font-size":   "10",
				"font-family": "system-ui, sans-serif",
			})
		}
	}

	return svg.Build(), nil
}

func (c *LineChart) buildLinearPath(points []struct{ x, y float64 }) string {
	var parts []string
	for i, p := range points {
		if i == 0 {
			parts = append(parts, fmt.Sprintf("M %.2f %.2f", p.x, p.y))
		} else {
			parts = append(parts, fmt.Sprintf("L %.2f %.2f", p.x, p.y))
		}
	}
	return strings.Join(parts, " ")
}

func (c *LineChart) buildSmoothPath(points []struct{ x, y float64 }) string {
	if len(points) < 2 {
		return ""
	}

	// Use monotone cubic interpolation to prevent overshoots
	// This ensures the curve stays within the data bounds
	var parts []string
	parts = append(parts, fmt.Sprintf("M %.2f %.2f", points[0].x, points[0].y))

	if len(points) == 2 {
		// Just two points - draw a line
		parts = append(parts, fmt.Sprintf("L %.2f %.2f", points[1].x, points[1].y))
		return strings.Join(parts, " ")
	}

	// Calculate tangents for monotone cubic spline
	n := len(points)
	tangents := make([]float64, n)

	// Calculate slopes between points
	slopes := make([]float64, n-1)
	for i := 0; i < n-1; i++ {
		dx := points[i+1].x - points[i].x
		if dx == 0 {
			slopes[i] = 0
		} else {
			slopes[i] = (points[i+1].y - points[i].y) / dx
		}
	}

	// Calculate tangents using monotone method
	tangents[0] = slopes[0]
	tangents[n-1] = slopes[n-2]

	for i := 1; i < n-1; i++ {
		if slopes[i-1]*slopes[i] <= 0 {
			// Different signs or zero - set tangent to 0 to prevent overshoot
			tangents[i] = 0
		} else {
			// Average the slopes, but limit to prevent overshoot
			tangents[i] = (slopes[i-1] + slopes[i]) / 2

			// Limit tangent to ensure monotonicity
			maxSlope := 3 * min(abs(slopes[i-1]), abs(slopes[i]))
			if abs(tangents[i]) > maxSlope {
				if tangents[i] > 0 {
					tangents[i] = maxSlope
				} else {
					tangents[i] = -maxSlope
				}
			}
		}
	}

	// Build cubic bezier path
	for i := 0; i < n-1; i++ {
		p0 := points[i]
		p1 := points[i+1]
		dx := (p1.x - p0.x) / 3

		// Control points
		cp1x := p0.x + dx
		cp1y := p0.y + tangents[i]*dx
		cp2x := p1.x - dx
		cp2y := p1.y - tangents[i+1]*dx

		parts = append(parts, fmt.Sprintf("C %.2f %.2f %.2f %.2f %.2f %.2f",
			cp1x, cp1y, cp2x, cp2y, p1.x, p1.y))
	}

	return strings.Join(parts, " ")
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (c *LineChart) getSeriesColor(index int) string {
	if index < len(c.series) && c.series[index].Color != "" {
		return c.series[index].Color
	}
	return c.config.Theme.GetSeriesColor(index)
}
