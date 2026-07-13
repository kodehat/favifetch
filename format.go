package favifetch

import (
	"bytes"
	"strconv"
	"strings"
)

// detectFormat detects the image format from the buffer and optional hints.
func detectFormat(data []byte, formatHint, mimeHint string) string {
	if len(data) < 2 {
		return ""
	}

	// Check magic bytes
	switch {
	// PNG: 89 50 4E 47
	case len(data) >= 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47:
		return "png"

	// JPEG: FF D8
	case data[0] == 0xFF && data[1] == 0xD8:
		return "jpg"

	// GIF: 47 49 46
	case len(data) >= 3 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46:
		return "gif"

	// ICO: 00 00 01 00
	case len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 && data[3] == 0x00:
		return "ico"

	// BMP: 42 4D
	case data[0] == 0x42 && data[1] == 0x4D:
		return "bmp"

	// WebP: RIFF .... WEBP
	case len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50:
		return "webp"

	// SVG: check for <svg tag (scan all data — SVGs can have long XML prologs)
	case bytes.Contains(bytes.ToLower(data), []byte("<svg")):
		return "svg"
	}

	// Fallback to format hint
	if formatHint != "" {
		hint := strings.ToLower(formatHint)
		if strings.Contains(hint, "svg") {
			return "svg"
		}
		if strings.Contains(hint, "png") {
			return "png"
		}
		if strings.Contains(hint, "jpeg") || strings.Contains(hint, "jpg") {
			return "jpg"
		}
		if strings.Contains(hint, "webp") {
			return "webp"
		}
		if strings.Contains(hint, "gif") {
			return "gif"
		}
		if strings.Contains(hint, "ico") || strings.Contains(hint, "icon") {
			return "ico"
		}
	}

	// Fallback to MIME hint
	if mimeHint != "" {
		hint := strings.ToLower(mimeHint)
		if strings.Contains(hint, "svg") {
			return "svg"
		}
		if strings.Contains(hint, "png") {
			return "png"
		}
		if strings.Contains(hint, "jpeg") || strings.Contains(hint, "jpg") {
			return "jpg"
		}
		if strings.Contains(hint, "webp") {
			return "webp"
		}
		if strings.Contains(hint, "gif") {
			return "gif"
		}
		if strings.Contains(hint, "ico") || strings.Contains(hint, "icon") {
			return "ico"
		}
	}

	// Default fallback
	return "png"
}

// detectDimensions returns the width and height of an image.
func detectDimensions(data []byte, format string) (int, int) {
	if format == "svg" {
		return extractSVGDimensions(data)
	}

	// For raster formats, use decodeDimensions from processor.go
	return decodeDimensions(data, format)
}

// extractSVGDimensions extracts width/height from an SVG tag.
func extractSVGDimensions(data []byte) (int, int) {
	content := string(data)

	// Find <svg tag
	svgIdx := strings.Index(strings.ToLower(content), "<svg")
	if svgIdx < 0 {
		return 0, 0
	}

	closeIdx := strings.Index(content[svgIdx:], ">")
	if closeIdx < 0 {
		return 0, 0
	}
	svgTag := content[svgIdx : svgIdx+closeIdx]

	// Try width/height attributes
	w := extractIntAttr(svgTag, "width")
	h := extractIntAttr(svgTag, "height")
	if w > 0 && h > 0 {
		return w, h
	}

	// Try viewBox="minX minY width height"
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

// extractIntAttr extracts an integer attribute from an XML tag.
// Handles attr="value" and attr='value' syntax.
func extractIntAttr(tag, attr string) int {
	// Try double quotes
	search := attr + `="`
	if idx := strings.Index(tag, search); idx >= 0 {
		rest := tag[idx+len(search):]
		if end := strings.IndexByte(rest, '"'); end >= 0 {
			if val, err := strconv.Atoi(rest[:end]); err == nil {
				return val
			}
		}
	}
	// Try single quotes
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
