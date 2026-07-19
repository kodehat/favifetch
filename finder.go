package favifetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// source types for favicons.
const (
	sourceLinkTag     = "link-tag"
	sourceManifest    = "manifest"
	sourceFallback    = "fallback"
	sourceFallbackAPI = "fallback-api"
)

// faviconSource represents a discovered favicon URL with metadata.
type faviconSource struct {
	URL    string
	Size   int
	Format string
	Source string
	Score  int
	order  int
}

// sortByScore sorts favicon sources by score descending.
func sortByScore(sources []faviconSource) {
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Score > sources[j].Score
	})
}

// calculateScore computes a quality score for a favicon.
func calculateScore(size int, format, rel string, prefs []DetectedFormat) int {
	score := 50

	format = strings.ToLower(format)
	rel = strings.ToLower(rel)

	if len(prefs) > 0 {
		// User-specified format preferences: apply tiered bonus.
		score += formatPreferenceBonus(format, prefs)
	} else {
		// Default legacy format bonuses.
		if strings.Contains(format, "svg") {
			score += 100
		}
		switch {
		case strings.Contains(format, "png"):
			score += 20
		case strings.Contains(format, "webp"):
			score += 15
		case strings.Contains(format, "gif"):
			score += 10
		case strings.Contains(format, "ico"):
			score += 5
		case strings.Contains(format, "icon"): // x-icon etc
			score += 5
		}
	}

	// Size preference (larger is better) — applies regardless of preference mode.
	switch {
	case size >= 512:
		score += 90
	case size >= 256:
		score += 80
	case size >= 192:
		score += 70
	case size >= 128:
		score += 60
	case size >= 64:
		score += 50
	case size >= 32:
		score += 40
	}

	// Rel attribute preference — applies regardless of preference mode.
	if strings.Contains(rel, "apple-touch-icon") {
		score += 10
	}
	if strings.Contains(rel, "mask-icon") {
		score -= 10 // Usually monochrome
	}

	return score
}

// formatPreferenceBonus returns a score bonus based on where the format hint
// falls in the user's preference list. Higher-ranked formats get larger bonuses.
func formatPreferenceBonus(formatHint string, prefs []DetectedFormat) int {
	detected := detectFormatFromHint(formatHint)
	if detected == FormatUnknown {
		return 0
	}
	for i, pref := range prefs {
		if pref == detected {
			// Tier 0 = 1000, tier 1 = 800, tier 2 = 600, etc.
			return 1000 - i*200
		}
	}
	// Not in preference list — still valid, but gets no bonus.
	return 0
}

// resolveURL resolves a potentially relative URL against a base URL.
func resolveURL(rawURL, baseURL string) string {
	// Handle data URLs (inline images)
	if isDataURL(rawURL) {
		return rawURL
	}

	// Already absolute with scheme
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}

	// protocol-relative URL
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}

	// Resolve against base
	base, err := url.Parse(baseURL)
	if err != nil {
		return rawURL
	}
	ref, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return base.ResolveReference(ref).String()
}

// fetchBestFavicon tries to fetch favicons in score order and returns the first valid one.
func fetchBestFavicon(ctx context.Context, client *http.Client, sources []faviconSource, opts *Options) *FaviconResult {
	userAgent := opts.UserAgent
	if opts.Mode == ModeBrowser {
		userAgent = browserUserAgent
	}

	for _, src := range sources {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var data []byte
		var mimeType string
		var err error

		if isDataURL(src.URL) {
			data, mimeType, err = parseDataURL(src.URL)
		} else {
			data, err = fetchImage(ctx, client, src.URL, userAgent)
			if err == nil && len(data) > 0 {
				mimeType = detectMimeType(data)
			}
		}

		if err != nil || len(data) == 0 {
			continue
		}

		if err := validateImageData(data, opts.MaxImageSize); err != nil {
			continue
		}

		format := detectFormat(data, src.Format, mimeType)
		if format == FormatUnknown || !hasValidImageMagic(data, format) {
			continue
		}

		w, h := detectDimensions(data, format)

		return &FaviconResult{
			Data:      data,
			Format:    format,
			Width:     w,
			Height:    h,
			Source:    src.Source,
			SourceURL: src.URL,
			Size:      len(data),
		}
	}
	return nil
}

// hasValidImageMagic checks that the data has valid image magic bytes or is SVG.
func hasValidImageMagic(data []byte, format DetectedFormat) bool {
	if format == FormatSVG {
		return bytes.Contains(bytes.ToLower(data), []byte("<svg"))
	}
	if len(data) < 2 {
		return false
	}
	switch {
	case len(data) >= 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47:
		return format == FormatPNG
	case data[0] == 0xFF && data[1] == 0xD8:
		return format == FormatJPEG
	case len(data) >= 3 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46:
		return format == FormatGIF
	case len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 && data[3] == 0x00:
		return format == FormatICO
	case data[0] == 0x42 && data[1] == 0x4D:
		return format == FormatBMP
	case len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50:
		return format == FormatWebP
	default:
		return false
	}
}

// fetchImage fetches an image from a URL with the given User-Agent.
func fetchImage(ctx context.Context, client *http.Client, urlStr, userAgent string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("image fetch HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit per image
	if err != nil {
		return nil, err
	}

	return data, nil
}

// validateImageData checks that the image buffer is valid and within size limits.
func validateImageData(data []byte, maxSize int64) error {
	if len(data) == 0 {
		return fmt.Errorf("empty image data")
	}
	if maxSize > 0 && int64(len(data)) > maxSize {
		return fmt.Errorf("image too large: %d bytes (max %d)", len(data), maxSize)
	}
	return nil
}

// detectMimeType returns the MIME type from magic bytes or returns empty.
func detectMimeType(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	format := detectFormat(data, "", "")
	switch format {
	case FormatPNG:
		return "image/png"
	case FormatJPEG:
		return "image/jpeg"
	case FormatWebP:
		return "image/webp"
	case FormatGIF:
		return "image/gif"
	case FormatICO:
		return "image/x-icon"
	case FormatSVG:
		return "image/svg+xml"
	case FormatBMP:
		return "image/bmp"
	default:
		return ""
	}
}
