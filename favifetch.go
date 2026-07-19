// Package favifetch discovers and fetches website favicons.
//
// It automatically finds the best favicon from a website by checking
// HTML <link> tags, web app manifests, common fallback locations,
// and optionally Google's favicon API.
//
// Basic usage:
//
//	result, err := favifetch.Fetch(ctx, "github.com")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Format: %s, Size: %d bytes, Source: %s\n",
//	    result.Format, result.Size, result.Source)
//	// result.Data contains the raw image bytes
package favifetch

import (
	"context"
	"encoding/base64"
	"fmt"
)

// FaviconResult holds the fetched favicon image data and metadata.
type FaviconResult struct {
	// Data is the raw image bytes (PNG, SVG, ICO, etc.)
	Data []byte

	// Format is the detected image format.
	Format DetectedFormat

	// Width and Height are the image dimensions in pixels.
	Width, Height int

	// Source indicates where the favicon was found:
	// "link-tag", "manifest", "fallback", "fallback-api"
	Source string

	// SourceURL is the original URL from which the favicon was fetched.
	SourceURL string

	// Size is the size of Data in bytes.
	Size int
}

// Fetch discovers and fetches the best favicon for the given URL.
//
// The url parameter can be a full URL ("https://github.com") or just a domain
// ("github.com"). If no scheme is present, https:// is prepended.
//
// Options can be provided to customize behavior (timeout, size, format, etc.).
// If no options are given, sensible defaults are used.
func Fetch(ctx context.Context, rawURL string, opts ...Option) (*FaviconResult, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	if options.Mode == ModeBrowser && (options.Size > 0 || options.Format != TargetUnspecified) {
		return nil, fmt.Errorf("favifetch: browser mode returns original favicon bytes and cannot be used with WithSize or WithFormat")
	}

	// Apply domain mappings
	rawURL = resolveDomainMapping(rawURL)

	// Validate and normalize the URL
	normalized, parsed, err := validateURL(rawURL, options.BlockPrivateIps)
	if err != nil {
		return nil, err
	}

	client := options.httpClient()

	// Create a context with the configured timeout
	ctx, cancel := context.WithTimeout(ctx, options.RequestTimeout)
	defer cancel()

	// Discover favicon sources
	sources, err := discoverFavicons(ctx, normalized, options)
	if err != nil {
		return nil, fmt.Errorf("favifetch: discovery failed for %s: %w", parsed.Host, err)
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("favifetch: no favicon sources found for %s", parsed.Host)
	}

	// Fetch the best favicon
	result := fetchBestFavicon(ctx, client, sources, options)
	if result == nil {
		return nil, fmt.Errorf("favifetch: could not fetch any favicon for %s", parsed.Host)
	}

	// Process the image (resize, format convert) if requested
	if options.Size > 0 || options.Format != TargetUnspecified {
		processed, newFormat, w, h, err := processImage(result.Data, options)
		if err == nil {
			result.Data = processed
			result.Format = newFormat
			result.Width = w
			result.Height = h
			result.Size = len(processed)
		}
		// If processing fails, return the original
	}

	return result, nil
}

// Discover returns all discovered favicon sources for a URL without fetching any image.
// This is useful for inspection/debugging.
func Discover(ctx context.Context, rawURL string, opts ...Option) ([]DiscoveredSource, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	rawURL = resolveDomainMapping(rawURL)
	normalized, _, err := validateURL(rawURL, options.BlockPrivateIps)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, options.RequestTimeout)
	defer cancel()

	sources, err := discoverFavicons(ctx, normalized, options)
	if err != nil {
		return nil, err
	}

	result := make([]DiscoveredSource, len(sources))
	for i, s := range sources {
		result[i] = DiscoveredSource{
			URL:    s.URL,
			Size:   s.Size,
			Format: detectFormatFromHint(s.Format),
			Source: s.Source,
			Score:  s.Score,
		}
	}
	return result, nil
}

// DiscoveredSource represents a favicon URL found during discovery.
type DiscoveredSource struct {
	URL    string
	Size   int
	Format DetectedFormat
	Source string
	Score  int
}

// Error types.

type errInvalidURL struct {
	rawURL string
	reason string
}

func (e errInvalidURL) Error() string {
	return fmt.Sprintf("favifetch: invalid URL %q: %s", e.rawURL, e.reason)
}

var errMissingURL = errInvalidURL{rawURL: "", reason: "empty URL"}

// base64Decode decodes a base64 string (handles both standard and URL-safe encoding).
func base64Decode(s string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return base64.RawURLEncoding.DecodeString(s)
	}
	return data, nil
}
