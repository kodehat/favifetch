package favifetch

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// DetectedFormat represents an image format detected from actual image data.
type DetectedFormat int

const (
	FormatUnknown DetectedFormat = iota
	FormatPNG
	FormatJPEG
	FormatSVG
	FormatICO
	FormatWebP
	FormatGIF
	FormatBMP
)

func (f DetectedFormat) String() string {
	switch f {
	case FormatPNG:
		return "png"
	case FormatJPEG:
		return "jpg"
	case FormatSVG:
		return "svg"
	case FormatICO:
		return "ico"
	case FormatWebP:
		return "webp"
	case FormatGIF:
		return "gif"
	case FormatBMP:
		return "bmp"
	default:
		return ""
	}
}

// IsWritable returns true if the format can be encoded to (PNG, JPEG, WebP).
func (f DetectedFormat) IsWritable() bool {
	switch f {
	case FormatPNG, FormatJPEG, FormatWebP:
		return true
	default:
		return false
	}
}

// Target converts to a TargetFormat if the format is writable.
// The bool is false for non-writable formats (SVG, ICO, GIF, BMP, Unknown).
func (f DetectedFormat) Target() (TargetFormat, bool) {
	switch f {
	case FormatPNG:
		return TargetPNG, true
	case FormatJPEG:
		return TargetJPEG, true
	case FormatWebP:
		return TargetWebP, true
	default:
		return TargetUnspecified, false
	}
}

// TargetFormat represents an output format for encoding (PNG, JPEG, WebP).
type TargetFormat int

const (
	TargetUnspecified TargetFormat = iota
	TargetPNG
	TargetJPEG
	TargetWebP
)

func (t TargetFormat) String() string {
	switch t {
	case TargetPNG:
		return "png"
	case TargetJPEG:
		return "jpg"
	case TargetWebP:
		return "webp"
	default:
		return ""
	}
}

// Detected converts TargetFormat to its corresponding DetectedFormat.
func (t TargetFormat) Detected() DetectedFormat {
	switch t {
	case TargetPNG:
		return FormatPNG
	case TargetJPEG:
		return FormatJPEG
	case TargetWebP:
		return FormatWebP
	default:
		return FormatUnknown
	}
}

// ParseTargetFormat parses a string into a TargetFormat.
// Accepts "png", "jpg", "jpeg", "webp" (case-insensitive).
func ParseTargetFormat(s string) (TargetFormat, error) {
	switch strings.ToLower(s) {
	case "png":
		return TargetPNG, nil
	case "jpg", "jpeg":
		return TargetJPEG, nil
	case "webp":
		return TargetWebP, nil
	default:
		return TargetUnspecified, fmt.Errorf("favifetch: unsupported target format %q (valid: png, jpg, webp)", s)
	}
}

// detectFormat detects the image format from the buffer and optional hints.
func detectFormat(data []byte, formatHint, mimeHint string) DetectedFormat {
	if len(data) < 2 {
		return FormatUnknown
	}

	switch {
	case len(data) >= 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47:
		return FormatPNG
	case data[0] == 0xFF && data[1] == 0xD8:
		return FormatJPEG
	case len(data) >= 3 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46:
		return FormatGIF
	case len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 && data[3] == 0x00:
		return FormatICO
	case data[0] == 0x42 && data[1] == 0x4D:
		return FormatBMP
	case len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50:
		return FormatWebP
	case bytes.Contains(bytes.ToLower(data), []byte("<svg")):
		return FormatSVG
	}

	// Fallback to format hint
	if f := detectFormatFromHint(formatHint); f != FormatUnknown {
		return f
	}

	// Fallback to MIME hint
	if f := detectFormatFromHint(mimeHint); f != FormatUnknown {
		return f
	}

	return FormatUnknown
}

// detectFormatFromHint tries to determine the format from a hint string (file extension, MIME type, etc).
func detectFormatFromHint(hint string) DetectedFormat {
	hint = strings.ToLower(hint)
	if hint == "" {
		return FormatUnknown
	}
	if strings.Contains(hint, "svg") {
		return FormatSVG
	}
	if strings.Contains(hint, "png") {
		return FormatPNG
	}
	if strings.Contains(hint, "jpeg") || strings.Contains(hint, "jpg") {
		return FormatJPEG
	}
	if strings.Contains(hint, "webp") {
		return FormatWebP
	}
	if strings.Contains(hint, "gif") {
		return FormatGIF
	}
	if strings.Contains(hint, "ico") || strings.Contains(hint, "icon") {
		return FormatICO
	}
	return FormatUnknown
}

func detectDimensions(data []byte, format DetectedFormat) (int, int) {
	if format == FormatSVG {
		return extractSVGDimensions(data)
	}
	return decodeDimensions(data)
}

func extractSVGDimensions(data []byte) (int, int) {
	content := string(data)
	svgIdx := strings.Index(strings.ToLower(content), "<svg")
	if svgIdx < 0 {
		return 0, 0
	}
	closeIdx := strings.Index(content[svgIdx:], ">")
	if closeIdx < 0 {
		return 0, 0
	}
	svgTag := content[svgIdx : svgIdx+closeIdx]

	w := extractIntAttr(svgTag, "width")
	h := extractIntAttr(svgTag, "height")
	if w > 0 && h > 0 {
		return w, h
	}

	vbIdx := strings.Index(svgTag, "viewBox")
	if vbIdx >= 0 {
		rest := svgTag[vbIdx+7:]
		if eqIdx := strings.Index(rest, "="); eqIdx >= 0 {
			rest = rest[eqIdx+1:]
			rest = strings.TrimLeft(rest, "\"'")
			closing := strings.IndexAny(rest, "\"'>")
			if closing > 0 {
				rest = rest[:closing]
			}
			parts := strings.Fields(rest)
			if len(parts) >= 4 {
				vbW, errW := strconv.ParseFloat(parts[2], 64)
				vbH, errH := strconv.ParseFloat(parts[3], 64)
				if errW == nil && errH == nil {
					return int(vbW), int(vbH)
				}
			}
		}
	}
	return 0, 0
}

func extractIntAttr(tag, attr string) int {
	search := attr + `="`
	if idx := strings.Index(tag, search); idx >= 0 {
		rest := tag[idx+len(search):]
		if end := strings.IndexByte(rest, '"'); end >= 0 {
			if val, err := strconv.Atoi(rest[:end]); err == nil {
				return val
			}
		}
	}
	search = attr + `='`
	if idx := strings.Index(tag, search); idx >= 0 {
		rest := tag[idx+len(search):]
		if end := strings.IndexByte(rest, '\''); end >= 0 {
			if val, err := strconv.Atoi(rest[:end]); err == nil {
				return val
			}
		}
	}
	return 0
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
