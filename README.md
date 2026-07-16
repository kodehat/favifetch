# favifetch

> **Based on [Vemetric/favicon-api](https://github.com/Vemetric/favicon-api)** — a TypeScript/Bun favicon API service.  
> `favifetch` is the Go library version of the same concept.

A Go library for discovering and fetching website favicons. Automatically finds the best favicon from HTML `<link>` tags, web app manifests, common fallback locations, and optionally Google's favicon API.

## Features

- **Smart Discovery** — Finds favicons from `<link rel="icon">`, `<link rel="apple-touch-icon">`, `manifest.json`, and common fallback paths (`/favicon.ico`, `/apple-touch-icon.png`)
- **Quality Ranking** — Scores favicons by size, format, and source, returning the best one first
- **Format Detection** — Detects PNG, JPEG, GIF, ICO, WebP, SVG, BMP via magic bytes
- **Image Processing** — Resize to any pixel size and convert between PNG, JPG, WebP
- **SSRF Protection** — Blocks requests to private IPs by default
- **Domain Mappings** — Maps app package names (e.g. `com.pinterest`) to canonical domains
- **Google API Fallback** — Optionally uses Google's `s2/favicons` as a last-resort source
- **Context Support** — Full `context.Context` integration for cancellation and timeouts
- **Zero cgo** — Pure Go image processing (no Sharp/Node dependency)

## Installation

```bash
go get github.com/kodehat/favifetch
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/kodehat/favifetch"
)

func main() {
    ctx := context.Background()

    // Fetch a favicon
    result, err := favifetch.Fetch(ctx, "github.com")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("Format:  %s\n", result.Format) // png, svg, etc. (implements Stringer)
    fmt.Printf("Size:    %dx%d (%d bytes)\n", result.Width, result.Height, result.Size)
    fmt.Printf("Source:  %s\n", result.Source)
    fmt.Printf("URL:     %s\n", result.SourceURL)

    // result.Data contains the raw image bytes
    // os.WriteFile("favicon."+result.Format, result.Data, 0644)
}
```

## Usage

### Basic Fetch

```go
result, err := favifetch.Fetch(ctx, "github.com")
// result.Data → raw image bytes
// result.Format → "svg", "png", "ico", etc.
```

URLs without a scheme default to `https://`. Full URLs with paths are also supported:

```go
result, err := favifetch.Fetch(ctx, "https://example.com/blog/post")
```

### With Options

Resize to 128×128 and convert to PNG:

```go
result, err := favifetch.Fetch(ctx, "github.com",
    favifetch.WithSize(128),
    favifetch.WithFormat(favifetch.TargetPNG),
)
```

Custom timeout and user agent:

```go
result, err := favifetch.Fetch(ctx, "example.com",
    favifetch.WithTimeout(10*time.Second),
    favifetch.WithUserAgent("MyApp/2.0"),
)
```

Disable Google API fallback and SSRF protection:

```go
result, err := favifetch.Fetch(ctx, "internal.company.com",
    favifetch.WithFallbackAPI(false),
    favifetch.WithBlockPrivateIPs(false),
)
```

### Discovery Only

List all discovered favicon sources without fetching any image:

```go
sources, err := favifetch.Discover(ctx, "github.com")
for _, s := range sources {
    fmt.Printf("[%s] score=%d size=%d %s\n", s.Source, s.Score, s.Size, s.URL)
}
// Output:
// [link-tag] score=150 size=0 https://github.githubassets.com/favicons/favicon.svg
// [link-tag] score=70 size=32 https://github.githubassets.com/favicons/favicon.png
// [manifest] score=40 size=192 https://github.githubassets.com/assets/icon-192.png
// [fallback] score=20 https://github.com/apple-touch-icon.png
// [fallback] score=10 https://github.com/favicon.ico
// [fallback-api] score=1 https://www.google.com/s2/favicons?...
```

## API Reference

### `Fetch(ctx, url, opts...) (*FaviconResult, error)`

Discovers and fetches the best favicon for the given URL.

### `Discover(ctx, url, opts...) ([]DiscoveredSource, error)`

Returns all discovered favicon sources without fetching any image.

### Types

```go
type FaviconResult struct {
    Data      []byte          // Raw image bytes
    Format    DetectedFormat  // png, jpg, svg, ico, webp, gif, bmp
    Width     int             // Image width in pixels
    Height    int             // Image height in pixels
    Source    string          // "link-tag", "manifest", "fallback", "fallback-api"
    SourceURL string          // Original URL the favicon was fetched from
    Size      int             // Size of Data in bytes
}

type DiscoveredSource struct {
    URL    string
    Size   int
    Format DetectedFormat
    Source string
    Score  int
}
```

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithUserAgent(s)` | `"Favifetch/1.0"` | User-Agent header |
| `WithTimeout(d)` | `5s` | Request timeout |
| `WithMaxImageSize(n)` | `5MB` | Max favicon size to accept |
| `WithMaxRedirects(n)` | `5` | Max HTTP redirects |
| `WithBlockPrivateIPs(bool)` | `true` | Block private IP ranges |
| `WithFallbackAPI(bool)` | `true` | Use Google favicon API fallback |
| `WithSize(px)` | `0` | Resize to px×px (0 = no resize) |
| `WithFormat(f)` | `TargetUnspecified` | Convert to `TargetPNG`, `TargetJPEG`, or `TargetWebP` |
| `WithHTTPClient(c)` | `http.DefaultClient` | Custom HTTP client |
| `WithPreferredFormats(f...)` | (default order) | Set preferred favicon formats in priority order |

## Favicon Sources & Scoring

The library searches these sources in priority order:

1. **HTML `<link>` tags** — `<link rel="icon">`, `<link rel="apple-touch-icon">`, `<link rel="mask-icon">`
2. **Web App Manifest** — `manifest.json` icons array
3. **Common fallbacks** — `/apple-touch-icon.png` (score 20), `/favicon.ico` (score 10)
4. **Google API** — `google.com/s2/favicons` (score 1, optional)

Scoring weights SVG (+100), large sizes (+90 for ≥512px), PNG (+20), WebP (+15), and apple-touch-icon (+10). Mask icons are penalized (−10).

You can customize format priorities with `WithPreferredFormats` (see below).

## Format Preferences

The `WithPreferredFormats` option lets you control which favicon formats are preferred:

```go
// Prefer SVG, fall back to PNG, then ICO
result, err := favifetch.Fetch(ctx, "example.com",
    favifetch.WithPreferredFormats(favifetch.FormatSVG, favifetch.FormatPNG, favifetch.FormatICO),
)
```

When preferences are set, sources matching a higher-ranked format get a large score bonus, ensuring they are tried first. Unlisted formats are still tried as a last resort if no preferred format succeeds. Within the same format tier, larger sizes and better sources still win.

```go
// Prefer PNG only — skip SVG entirely unless nothing else works
result, err := favifetch.Fetch(ctx, "example.com",
    favifetch.WithPreferredFormats(favifetch.FormatPNG),
)
```

When no preferences are set (or `nil`), the default order is: SVG > PNG > WebP > JPEG > ICO > GIF > BMP.

## Error Types

- `favifetch.ErrPrivateIP` — returned when the target URL resolves to a private IP and `BlockPrivateIPs` is enabled
- `favifetch.errInvalidURL` — returned for malformed URLs or unsupported schemes

## Credits

Based on [Vemetric/favicon-api](https://github.com/Vemetric/favicon-api), a TypeScript/Bun favicon API service by [Vemetric](https://vemetric.com).

Built with:
- [`golang.org/x/net/html`](https://pkg.go.dev/golang.org/x/net/html) — HTML parsing
- [`golang.org/x/image`](https://pkg.go.dev/golang.org/x/image) — Image resampling
- [`github.com/HugoSmits86/nativewebp`](https://github.com/HugoSmits86/nativewebp) — WebP encode/decode
- [`github.com/srwiley/oksvg`](https://github.com/srwiley/oksvg) + [`github.com/srwiley/rasterx`](https://github.com/srwiley/rasterx) — SVG rasterization

## License

MIT
