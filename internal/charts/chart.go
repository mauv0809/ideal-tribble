// Package charts provides a server-side SVG chart engine optimized for htmx/templ.
//
// Design principles:
// - Zero JavaScript required for basic charts
// - SVG output embeds directly in HTML via templ
// - htmx can swap chart containers seamlessly
// - Extensible for future interactivity via data attributes
// - Theme-aware (Catppuccin color palette)
package charts

import (
	"html/template"
)

// Chart is the main interface that all chart types implement.
type Chart interface {
	// Render generates the SVG as a template.HTML safe string.
	// The output can be directly embedded in templ templates.
	Render() (template.HTML, error)

	// SetDimensions sets the chart width and height in pixels.
	SetDimensions(width, height int) Chart

	// SetTheme applies a color theme to the chart.
	SetTheme(theme Theme) Chart

	// SetTitle sets the chart title.
	SetTitle(title string) Chart
}

// DataPoint represents a single data point with optional metadata.
type DataPoint struct {
	Label string  // X-axis label or category
	Value float64 // Y-axis value

	// Optional metadata for interactivity (rendered as data-* attributes)
	Meta map[string]string
}

// Series represents a data series for multi-series charts.
type Series struct {
	Name   string      // Series name (for legend)
	Points []DataPoint // Data points in this series
	Color  string      // Optional override color (hex)
}

// ChartConfig holds common configuration for all chart types.
type ChartConfig struct {
	Width  int
	Height int
	Title  string
	Theme  Theme

	// Padding around the chart area
	PaddingTop    int
	PaddingRight  int
	PaddingBottom int
	PaddingLeft   int

	// Axis configuration
	ShowXAxis   bool
	ShowYAxis   bool
	ShowGrid    bool
	XAxisLabel  string
	YAxisLabel  string

	// Legend
	ShowLegend    bool
	LegendPosition string // "top", "bottom", "left", "right"

	// Interactivity hooks (for future JS enhancement)
	EnableDataAttributes bool // Add data-* attributes to elements
	ChartID              string // Unique ID for targeting with JS
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() ChartConfig {
	return ChartConfig{
		Width:                400,
		Height:               250,
		Theme:                DefaultTheme(),
		PaddingTop:           40,
		PaddingRight:         20,
		PaddingBottom:        40,
		PaddingLeft:          50,
		ShowXAxis:            true,
		ShowYAxis:            true,
		ShowGrid:             true,
		ShowLegend:           false,
		LegendPosition:       "top",
		EnableDataAttributes: true,
	}
}

// Theme defines the color palette for charts.
type Theme struct {
	// Background colors
	Background    string
	ChartArea     string

	// Axis and grid
	AxisColor     string
	GridColor     string
	TextColor     string
	TextMuted     string

	// Data colors (for series)
	DataColors    []string

	// Semantic colors
	SuccessColor  string
	ErrorColor    string
	WarningColor  string
}

// DefaultTheme returns the Catppuccin Mocha-inspired theme.
func DefaultTheme() Theme {
	return Theme{
		Background:   "transparent",
		ChartArea:    "transparent",
		AxisColor:    "#6c7086", // Overlay0
		GridColor:    "#313244", // Surface0
		TextColor:    "#cdd6f4", // Text
		TextMuted:    "#6c7086", // Overlay0

		// Catppuccin accent colors
		DataColors: []string{
			"#89b4fa", // Blue
			"#a6e3a1", // Green
			"#f9e2af", // Yellow
			"#f38ba8", // Red
			"#cba6f7", // Mauve
			"#94e2d5", // Teal
			"#fab387", // Peach
			"#f5c2e7", // Pink
		},

		SuccessColor: "#a6e3a1", // Green
		ErrorColor:   "#f38ba8", // Red
		WarningColor: "#f9e2af", // Yellow
	}
}

// LightTheme returns a light mode theme (Catppuccin Latte-inspired).
func LightTheme() Theme {
	return Theme{
		Background:   "transparent",
		ChartArea:    "transparent",
		AxisColor:    "#9ca0b0", // Overlay0
		GridColor:    "#e6e9ef", // Surface0
		TextColor:    "#4c4f69", // Text
		TextMuted:    "#9ca0b0", // Overlay0

		DataColors: []string{
			"#1e66f5", // Blue
			"#40a02b", // Green
			"#df8e1d", // Yellow
			"#d20f39", // Red
			"#8839ef", // Mauve
			"#179299", // Teal
			"#fe640b", // Peach
			"#ea76cb", // Pink
		},

		SuccessColor: "#40a02b",
		ErrorColor:   "#d20f39",
		WarningColor: "#df8e1d",
	}
}

// Helper function to get color for a series index.
func (t Theme) GetSeriesColor(index int) string {
	if len(t.DataColors) == 0 {
		return "#89b4fa" // Fallback blue
	}
	return t.DataColors[index%len(t.DataColors)]
}
