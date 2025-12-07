package charts

import (
	"fmt"
	"html/template"
)

// BarChart renders a vertical bar chart.
type BarChart struct {
	config     ChartConfig
	series     []Series
	horizontal bool  // Horizontal bars instead of vertical
	stacked    bool  // Stack multiple series
	barGap     float64 // Gap between bars as percentage of bar width
	groupGap   float64 // Gap between groups as percentage of group width
}

// NewBarChart creates a new bar chart.
func NewBarChart() *BarChart {
	return &BarChart{
		config:   DefaultConfig(),
		series:   make([]Series, 0),
		barGap:   0.1,
		groupGap: 0.2,
	}
}

// SetDimensions sets the chart dimensions.
func (c *BarChart) SetDimensions(width, height int) Chart {
	c.config.Width = width
	c.config.Height = height
	return c
}

// SetTheme sets the chart theme.
func (c *BarChart) SetTheme(theme Theme) Chart {
	c.config.Theme = theme
	return c
}

// SetTitle sets the chart title.
func (c *BarChart) SetTitle(title string) Chart {
	c.config.Title = title
	return c
}

// SetConfig allows setting the full config.
func (c *BarChart) SetConfig(config ChartConfig) *BarChart {
	c.config = config
	return c
}

// AddSeries adds a data series to the chart.
func (c *BarChart) AddSeries(series Series) *BarChart {
	c.series = append(c.series, series)
	return c
}

// AddDataPoints is a convenience method to add a single series.
func (c *BarChart) AddDataPoints(name string, points []DataPoint) *BarChart {
	c.series = append(c.series, Series{
		Name:   name,
		Points: points,
	})
	return c
}

// Horizontal makes the bar chart horizontal.
func (c *BarChart) Horizontal(enabled bool) *BarChart {
	c.horizontal = enabled
	return c
}

// Stacked enables stacked bars for multiple series.
func (c *BarChart) Stacked(enabled bool) *BarChart {
	c.stacked = enabled
	return c
}

// Render generates the SVG.
func (c *BarChart) Render() (template.HTML, error) {
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
	numCategories := 0

	if c.stacked {
		// For stacked charts, sum values at each position
		if len(c.series) > 0 {
			numCategories = len(c.series[0].Points)
			for i := 0; i < numCategories; i++ {
				var sum float64 = 0
				for _, s := range c.series {
					if i < len(s.Points) {
						sum += s.Points[i].Value
					}
				}
				if sum > maxY {
					maxY = sum
				}
			}
		}
	} else {
		// Find max across all series
		for _, s := range c.series {
			if len(s.Points) > numCategories {
				numCategories = len(s.Points)
			}
			for _, p := range s.Points {
				if p.Value > maxY {
					maxY = p.Value
				}
				if p.Value < minY {
					minY = p.Value
				}
			}
		}
	}

	// Ensure minY starts at 0 for bar charts (typically)
	if minY > 0 {
		minY = 0
	}

	// Calculate nice scale
	minY, maxY, tickInterval := CalculateNiceScale(minY, maxY, 5)

	// Draw grid lines
	if c.config.ShowGrid {
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
		svg.Line(chartX, chartY, chartX, chartY+chartHeight, map[string]string{
			"stroke":       theme.AxisColor,
			"stroke-width": "1",
		})

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
		svg.Line(chartX, chartY+chartHeight, chartX+chartWidth, chartY+chartHeight, map[string]string{
			"stroke":       theme.AxisColor,
			"stroke-width": "1",
		})
	}

	// Calculate bar dimensions
	numSeries := len(c.series)
	groupWidth := chartWidth / float64(numCategories)
	groupPadding := groupWidth * c.groupGap

	var barWidth float64
	if c.stacked || numSeries == 1 {
		barWidth = groupWidth - groupPadding*2
	} else {
		barWidth = (groupWidth - groupPadding*2) / float64(numSeries)
		barWidth = barWidth * (1 - c.barGap)
	}

	// Draw bars
	for catIdx := 0; catIdx < numCategories; catIdx++ {
		groupX := chartX + float64(catIdx)*groupWidth + groupPadding

		var stackBase float64 = 0

		for seriesIdx, s := range c.series {
			if catIdx >= len(s.Points) {
				continue
			}

			p := s.Points[catIdx]
			color := c.getSeriesColor(seriesIdx)

			// Calculate bar position and size
			var barX, barY, barH float64

			if c.stacked {
				barX = groupX
				barH = (p.Value / (maxY - minY)) * chartHeight
				barY = chartY + chartHeight - stackBase - barH
				stackBase += barH
			} else {
				barX = groupX + float64(seriesIdx)*(barWidth/(1-c.barGap))
				barH = ((p.Value - minY) / (maxY - minY)) * chartHeight
				barY = chartY + chartHeight - barH
			}

			// Don't draw bars with zero height
			if barH < 1 {
				barH = 1
			}

			attrs := map[string]string{
				"fill": color,
				"rx":   "2", // Rounded corners
			}

			// Add data attributes for interactivity
			if c.config.EnableDataAttributes {
				attrs["data-series"] = s.Name
				attrs["data-index"] = fmt.Sprintf("%d", catIdx)
				attrs["data-value"] = fmt.Sprintf("%.2f", p.Value)
				attrs["data-label"] = p.Label
				attrs["class"] = "chart-bar"

				for k, v := range p.Meta {
					attrs["data-"+k] = v
				}
			}

			svg.Rect(barX, barY, barWidth, barH, attrs)
		}

		// X-axis label
		if c.config.ShowXAxis && len(c.series) > 0 && catIdx < len(c.series[0].Points) {
			labelX := groupX + (groupWidth-groupPadding*2)/2
			svg.Text(labelX, chartY+chartHeight+16, c.series[0].Points[catIdx].Label, map[string]string{
				"fill":        theme.TextMuted,
				"font-size":   "10",
				"text-anchor": "middle",
				"font-family": "system-ui, sans-serif",
			})
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

	// Draw legend below the chart (horizontal layout)
	if c.config.ShowLegend && len(c.series) > 1 {
		legendY := chartY + chartHeight + 28 // Below X-axis labels

		// Calculate total legend width for centering
		itemWidth := 70.0
		totalWidth := float64(len(c.series)) * itemWidth
		legendX := chartX + (chartWidth-totalWidth)/2
		if legendX < chartX {
			legendX = chartX
		}

		for i, s := range c.series {
			color := c.getSeriesColor(i)
			xOffset := float64(i) * itemWidth

			// Color marker (small square)
			svg.Rect(legendX+xOffset, legendY-6, 12, 12, map[string]string{
				"fill": color,
				"rx":   "2",
			})

			// Label
			svg.Text(legendX+xOffset+16, legendY+4, s.Name, map[string]string{
				"fill":        theme.TextColor,
				"font-size":   "10",
				"font-family": "system-ui, sans-serif",
			})
		}
	}

	return svg.Build(), nil
}

func (c *BarChart) getSeriesColor(index int) string {
	if index < len(c.series) && c.series[index].Color != "" {
		return c.series[index].Color
	}
	return c.config.Theme.GetSeriesColor(index)
}

// WinLossBarChart is a specialized bar chart for showing wins/losses.
type WinLossBarChart struct {
	*BarChart
}

// NewWinLossBarChart creates a bar chart pre-configured for win/loss display.
func NewWinLossBarChart() *WinLossBarChart {
	bc := NewBarChart()
	bc.config.ShowLegend = true
	return &WinLossBarChart{BarChart: bc}
}

// SetWinLossData sets data with wins and losses for each category.
func (c *WinLossBarChart) SetWinLossData(labels []string, wins, losses []int) *WinLossBarChart {
	theme := c.config.Theme

	winPoints := make([]DataPoint, len(labels))
	lossPoints := make([]DataPoint, len(labels))

	for i, label := range labels {
		winPoints[i] = DataPoint{Label: label, Value: float64(wins[i])}
		lossPoints[i] = DataPoint{Label: label, Value: float64(losses[i])}
	}

	c.AddSeries(Series{Name: "Wins", Points: winPoints, Color: theme.SuccessColor})
	c.AddSeries(Series{Name: "Losses", Points: lossPoints, Color: theme.ErrorColor})

	return c
}
