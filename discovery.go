package favifetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Browser-like User-Agent for HTML parsing (sites often block bots for HTML).
const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// discoverFavicons finds all possible favicon URLs for a given website.
func discoverFavicons(ctx context.Context, targetURL string, opts *Options) ([]faviconSource, error) {
	client := opts.httpClient()

	// Parse the URL to get the base URL
	baseURL, err := buildBaseURL(targetURL)
	if err != nil {
		return nil, err
	}

	// These will be populated by fetchHTML if successful
	finalBaseURL := baseURL

	// Fetch HTML and manifest in parallel
	type htmlResult struct {
		sources    []faviconSource
		newBaseURL string
		err        error
	}
	type manifestResult struct {
		sources []faviconSource
		err     error
	}

	htmlCh := make(chan htmlResult, 1)
	manifestCh := make(chan manifestResult, 1)

	go func() {
		sources, newBase, err := fetchAndParseHTML(ctx, client, targetURL, baseURL, opts)
		htmlCh <- htmlResult{sources: sources, newBaseURL: newBase, err: err}
	}()

	go func() {
		sources, err := fetchManifest(ctx, client, baseURL, opts)
		manifestCh <- manifestResult{sources: sources, err: err}
	}()

	htmlRes := <-htmlCh
	manifestRes := <-manifestCh

	// Update base URL to final URL after redirects
	if htmlRes.newBaseURL != "" {
		finalBaseURL = htmlRes.newBaseURL
	}

	var favicons []faviconSource

	// Add HTML-discovered favicons (ignore errors — we still try fallbacks)
	if htmlRes.err == nil {
		favicons = append(favicons, htmlRes.sources...)
	}

	// Add manifest-discovered favicons
	if manifestRes.err == nil {
		favicons = append(favicons, manifestRes.sources...)
	}

	// Always add common fallback locations
	favicons = append(favicons, faviconSource{
		URL:    finalBaseURL + "/favicon.ico",
		Source: sourceFallback,
		Score:  10 + formatPreferenceBonus("ico", opts.PreferredFormats),
	})
	favicons = append(favicons, faviconSource{
		URL:    finalBaseURL + "/apple-touch-icon.png",
		Source: sourceFallback,
		Score:  20 + formatPreferenceBonus("png", opts.PreferredFormats),
	})

	// Add Google's favicon API as last-resort fallback (if enabled)
	if opts.UseFallbackAPI {
		size := opts.Size
		if size == 0 {
			size = 64
		}
		host := strings.TrimPrefix(strings.TrimPrefix(targetURL, "https://"), "http://")
		host = strings.SplitN(host, "/", 2)[0]
		googleURL := fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=%d",
			url.QueryEscape("https://"+host), size)
		favicons = append(favicons, faviconSource{
			URL:    googleURL,
			Source: sourceFallbackAPI,
			Score:  1,
		})
	}

	// Sort by score (highest first)
	sortByScore(favicons)
	return favicons, nil
}

// fetchAndParseHTML fetches the HTML of a page and extracts favicon links.
// It first tries with the configured User-Agent, then falls back to a browser UA.
func fetchAndParseHTML(ctx context.Context, client *http.Client, targetURL, baseURL string, opts *Options) ([]faviconSource, string, error) {
	var htmlBody string
	var finalURL string
	var err error

	// Try with configured UA first
	htmlBody, finalURL, err = doFetchHTML(ctx, client, targetURL, opts.UserAgent)
	if err != nil {
		// Fallback: try with browser-like UA
		htmlBody, finalURL, err = doFetchHTML(ctx, client, targetURL, browserUserAgent)
	}
	if err != nil {
		return nil, "", err
	}

	// Parse final URL to get the real base
	parsed, _ := url.Parse(finalURL)
	var newBaseURL string
	if parsed != nil {
		newBaseURL = parsed.Scheme + "://" + parsed.Host
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		return nil, newBaseURL, err
	}

	sources := extractFaviconsFromHTML(doc, newBaseURL, baseURL, opts.PreferredFormats)
	return sources, newBaseURL, nil
}

// doFetchHTML performs a single HTTP GET to fetch HTML.
func doFetchHTML(ctx context.Context, client *http.Client, urlStr, userAgent string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return "", "", err
	}

	return string(body), resp.Request.URL.String(), nil
}

// extractFaviconsFromHTML parses an HTML document and extracts favicon URLs from <link> tags.
func extractFaviconsFromHTML(doc *html.Node, finalBaseURL, fallbackBaseURL string, prefs []DetectedFormat) []faviconSource {
	base := finalBaseURL
	if base == "" {
		base = fallbackBaseURL
	}

	var sources []faviconSource

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			var rel, href, sizes, typeAttr string
			for _, attr := range n.Attr {
				switch strings.ToLower(attr.Key) {
				case "rel":
					rel = attr.Val
				case "href":
					href = attr.Val
				case "sizes":
					sizes = attr.Val
				case "type":
					typeAttr = attr.Val
				}
			}

			// Check if this is a favicon-related link
			if href == "" {
				goto next
			}
			if !isFaviconRel(rel) {
				goto next
			}

			// Detect format from type or URL extension
			format := typeAttr
			if format == "" {
				if isDataURL(href) {
					if idx := strings.Index(href, "data:"); idx >= 0 {
						if semiIdx := strings.Index(href[idx:], ";"); semiIdx >= 0 {
							format = href[idx+5 : idx+semiIdx]
						}
					}
				} else if lastDot := strings.LastIndex(href, "."); lastDot >= 0 {
					format = href[lastDot+1:]
					if idx := strings.Index(format, "?"); idx >= 0 {
						format = format[:idx]
					}
				}
			}

			size := parseSize(sizes)
			score := calculateScore(size, format, rel, prefs)

			sources = append(sources, faviconSource{
				URL:    resolveURL(href, base),
				Size:   size,
				Format: format,
				Source: sourceLinkTag,
				Score:  score,
			})
		}
	next:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return sources
}

// isFaviconRel checks if a rel attribute indicates a favicon link.
func isFaviconRel(rel string) bool {
	rel = strings.ToLower(rel)
	parts := strings.Fields(rel)
	for _, p := range parts {
		if p == "icon" || p == "apple-touch-icon" || p == "apple-touch-icon-precomposed" || p == "mask-icon" || p == "shortcut icon" {
			return true
		}
	}
	return false
}

// fetchManifest fetches and parses a web app manifest.
func fetchManifest(ctx context.Context, client *http.Client, baseURL string, opts *Options) ([]faviconSource, error) {
	manifestURL := baseURL + "/manifest.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", opts.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("manifest HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // 1MB limit
	if err != nil {
		return nil, err
	}

	var manifest struct {
		Icons []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
			Type  string `json:"type"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, err
	}

	var sources []faviconSource
	for _, icon := range manifest.Icons {
		if icon.Src == "" {
			continue
		}
		sources = append(sources, faviconSource{
			URL:    resolveURL(icon.Src, baseURL),
			Size:   parseSize(icon.Sizes),
			Format: icon.Type,
			Source: sourceManifest,
			Score:  40 + formatPreferenceBonus(icon.Type, opts.PreferredFormats),
		})
	}
	return sources, nil
}

// parseSize parses a "WxH" size string and returns W.
func parseSize(sizes string) int {
	if sizes == "" {
		return 0
	}
	sizes = strings.ToLower(sizes)
	// "any" means scalable — treat as very large
	if sizes == "any" {
		return 9999
	}
	// Try "WxH" format
	var w, h int
	if n, err := fmt.Sscanf(sizes, "%dx%d", &w, &h); n == 2 && err == nil {
		return w
	}
	// Try single dimension
	if n, err := fmt.Sscanf(sizes, "%d", &w); n == 1 && err == nil {
		return w
	}
	return 0
}

// buildBaseURL constructs a base URL from a target URL string.
func buildBaseURL(rawURL string) (string, error) {
	normalized, _, err := validateURL(rawURL, false) // skip SSRF for base URL building
	if err != nil {
		return "", err
	}
	parsed, _ := url.Parse(normalized)
	if parsed == nil {
		return "", errInvalidURL{rawURL: rawURL, reason: "parse failed"}
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}
