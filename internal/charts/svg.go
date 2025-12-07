package charts

import (
	"fmt"
	"html/template"
	"strings"
)

// SVGBuilder helps construct SVG documents.
type SVGBuilder struct {
	width    int
	height   int
	elements []string
	defs     []string
}

// NewSVGBuilder creates a new SVG builder with the given dimensions.
func NewSVGBuilder(width, height int) *SVGBuilder {
	return &SVGBuilder{
		width:    width,
		height:   height,
		elements: make([]string, 0),
		defs:     make([]string, 0),
	}
}

// AddDef adds a definition (gradient, filter, etc.) to the defs section.
func (s *SVGBuilder) AddDef(def string) {
	s.defs = append(s.defs, def)
}

// AddElement adds an SVG element to the document.
func (s *SVGBuilder) AddElement(element string) {
	s.elements = append(s.elements, element)
}

// Rect adds a rectangle element.
func (s *SVGBuilder) Rect(x, y, width, height float64, attrs map[string]string) {
	s.AddElement(fmt.Sprintf(
		`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f"%s/>`,
		x, y, width, height, attrsToString(attrs),
	))
}

// Line adds a line element.
func (s *SVGBuilder) Line(x1, y1, x2, y2 float64, attrs map[string]string) {
	s.AddElement(fmt.Sprintf(
		`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f"%s/>`,
		x1, y1, x2, y2, attrsToString(attrs),
	))
}

// Circle adds a circle element.
func (s *SVGBuilder) Circle(cx, cy, r float64, attrs map[string]string) {
	s.AddElement(fmt.Sprintf(
		`<circle cx="%.2f" cy="%.2f" r="%.2f"%s/>`,
		cx, cy, r, attrsToString(attrs),
	))
}

// Path adds a path element.
func (s *SVGBuilder) Path(d string, attrs map[string]string) {
	s.AddElement(fmt.Sprintf(
		`<path d="%s"%s/>`,
		d, attrsToString(attrs),
	))
}

// Text adds a text element.
func (s *SVGBuilder) Text(x, y float64, content string, attrs map[string]string) {
	s.AddElement(fmt.Sprintf(
		`<text x="%.2f" y="%.2f"%s>%s</text>`,
		x, y, attrsToString(attrs), template.HTMLEscapeString(content),
	))
}

// Group starts a group with optional transform and attributes.
func (s *SVGBuilder) Group(attrs map[string]string, elements func()) {
	s.AddElement(fmt.Sprintf(`<g%s>`, attrsToString(attrs)))
	elements()
	s.AddElement(`</g>`)
}

// Build generates the final SVG string.
func (s *SVGBuilder) Build() template.HTML {
	var sb strings.Builder

	// SVG opening tag with viewBox for responsiveness
	sb.WriteString(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="100%%" height="100%%" preserveAspectRatio="xMidYMid meet" style="max-width: %dpx;">`,
		s.width, s.height, s.width,
	))

	// Defs section if we have any
	if len(s.defs) > 0 {
		sb.WriteString("<defs>")
		for _, def := range s.defs {
			sb.WriteString(def)
		}
		sb.WriteString("</defs>")
	}

	// All elements
	for _, el := range s.elements {
		sb.WriteString(el)
	}

	sb.WriteString("</svg>")

	return template.HTML(sb.String())
}

// Helper to convert attribute map to string.
func attrsToString(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}

	var parts []string
	for k, v := range attrs {
		parts = append(parts, fmt.Sprintf(` %s="%s"`, k, template.HTMLEscapeString(v)))
	}
	return strings.Join(parts, "")
}

// DataAttrs creates a map with data-* attributes for interactivity.
func DataAttrs(meta map[string]string) map[string]string {
	attrs := make(map[string]string)
	for k, v := range meta {
		attrs["data-"+k] = v
	}
	return attrs
}

// MergeAttrs merges multiple attribute maps.
func MergeAttrs(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// Axis helpers

// CalculateNiceScale calculates nice axis bounds and tick intervals.
func CalculateNiceScale(minVal, maxVal float64, maxTicks int) (min, max, tickInterval float64) {
	// Handle edge cases
	if minVal == maxVal {
		if minVal == 0 {
			return 0, 10, 2
		}
		minVal = minVal * 0.9
		maxVal = maxVal * 1.1
	}

	rangeVal := maxVal - minVal

	// Calculate rough tick interval
	roughInterval := rangeVal / float64(maxTicks-1)

	// Find nice interval (1, 2, 5, 10, 20, 50, etc.)
	magnitude := 1.0
	for roughInterval >= 10 {
		roughInterval /= 10
		magnitude *= 10
	}
	for roughInterval < 1 {
		roughInterval *= 10
		magnitude /= 10
	}

	// Round to nice values
	if roughInterval <= 1.5 {
		tickInterval = 1 * magnitude
	} else if roughInterval <= 3 {
		tickInterval = 2 * magnitude
	} else if roughInterval <= 7 {
		tickInterval = 5 * magnitude
	} else {
		tickInterval = 10 * magnitude
	}

	// Calculate bounds
	min = float64(int(minVal/tickInterval)) * tickInterval
	if min > minVal {
		min -= tickInterval
	}

	// Round max up to nearest tick interval, but don't add extra if already on boundary
	// Use small tolerance for floating point comparison (avoid adding tick for tiny differences)
	max = float64(int(maxVal/tickInterval)) * tickInterval
	if max < maxVal-0.0001 {
		max += tickInterval
	}

	// Ensure we don't go negative if data is all positive
	if minVal >= 0 && min < 0 {
		min = 0
	}

	return min, max, tickInterval
}

// FormatTickValue formats a tick value for display.
func FormatTickValue(value float64) string {
	if value == float64(int(value)) {
		return fmt.Sprintf("%d", int(value))
	}
	return fmt.Sprintf("%.1f", value)
}
