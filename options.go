package favifetch

import (
	"net/http"
	"time"
)

const defaultVemetricAPIHost = "favicon.vemetric.com"

// Options holds all configuration for the favicon fetcher.
type Options struct {
	// Mode controls how favicon candidates are selected. The default ModeBest
	// ranks all supported favicon sources by quality. ModeBrowser emulates
	// Chromium's regular tab-icon selection from the initial HTML document.
	Mode FaviconMode

	// UserAgent sent in HTTP requests for HTML and manifest fetching.
	// ModeBrowser always uses its Chromium-like User-Agent instead.
	UserAgent string

	// RequestTimeout is the maximum time for the entire fetch operation.
	RequestTimeout time.Duration

	// MaxImageSize is the maximum size (in bytes) of a favicon image to accept.
	MaxImageSize int64

	// MaxRedirects limits the number of HTTP redirects to follow.
	MaxRedirects int

	// BlockPrivateIps controls whether requests to private IP ranges are rejected.
	BlockPrivateIps bool

	// UseFallbackAPI enables the Vemetric favicon API as a last-resort fallback.
	UseFallbackAPI bool

	// VemetricAPIHost is the host, optionally including a port, of a
	// Vemetric-compatible favicon API. Requests use HTTPS.
	VemetricAPIHost string

	// Size is the desired output size in pixels (resizes favicon to size×size).
	// 0 means no resizing.
	Size int

	// Format is the desired output format. Zero value (TargetUnspecified) means keep original.
	Format TargetFormat

	// HTTPClient is the *http.Client to use for all requests. If nil, a default
	// client is created from the other options.
	HTTPClient *http.Client

	// PreferredFormats is an ordered list of preferred favicon formats.
	// The first format is most preferred, the last is least.
	// During discovery, sources matching a higher-preference format get a
	// large score bonus, so they are tried before lower-preference formats.
	// Unlisted formats are still tried as a last resort.
	// When nil or empty, the default preference order (SVG > PNG > WebP > JPEG > ICO > GIF > BMP) is used.
	PreferredFormats []DetectedFormat
}

// DefaultOptions returns the default configuration.
// Optional Option arguments can be passed to override defaults.
func DefaultOptions(opts ...Option) *Options {
	o := &Options{
		Mode:            ModeBest,
		UserAgent:       "Favifetch/1.0",
		RequestTimeout:  5 * time.Second,
		MaxImageSize:    5 * 1024 * 1024, // 5MB
		MaxRedirects:    5,
		BlockPrivateIps: true,
		UseFallbackAPI:  true,
		VemetricAPIHost: defaultVemetricAPIHost,
		Size:            0,
		Format:          TargetUnspecified,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Option is a functional option for configuring the fetcher.
type Option func(*Options)

// FaviconMode controls how favicon candidates are selected.
type FaviconMode int

const (
	// ModeBest selects the highest-ranked favicon from all supported sources.
	ModeBest FaviconMode = iota
	// ModeBrowser selects a Chromium-style regular tab favicon from the initial
	// HTML document. When enabled, the fallback API is used if those candidates
	// cannot be fetched. It returns the original image bytes and does not support
	// resizing or format conversion.
	ModeBrowser
)

// WithMode sets the favicon selection mode.
func WithMode(mode FaviconMode) Option {
	return func(o *Options) { o.Mode = mode }
}

// WithUserAgent sets the User-Agent header. It is ignored in ModeBrowser,
// which always uses its Chromium-like User-Agent.
func WithUserAgent(ua string) Option {
	return func(o *Options) { o.UserAgent = ua }
}

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option {
	return func(o *Options) { o.RequestTimeout = d }
}

// WithMaxImageSize sets the maximum favicon image size in bytes.
func WithMaxImageSize(size int64) Option {
	return func(o *Options) { o.MaxImageSize = size }
}

// WithMaxRedirects sets the maximum number of HTTP redirects.
func WithMaxRedirects(n int) Option {
	return func(o *Options) { o.MaxRedirects = n }
}

// WithBlockPrivateIPs enables or disables blocking of private IP ranges.
func WithBlockPrivateIPs(block bool) Option {
	return func(o *Options) { o.BlockPrivateIps = block }
}

// WithFallbackAPI enables or disables the Vemetric favicon API fallback.
func WithFallbackAPI(use bool) Option {
	return func(o *Options) { o.UseFallbackAPI = use }
}

// WithVemetricAPIHost sets the host, optionally including a port, for a
// self-hosted Vemetric-compatible favicon API. Requests use HTTPS.
func WithVemetricAPIHost(host string) Option {
	return func(o *Options) { o.VemetricAPIHost = host }
}

// WithSize sets the desired output size (resize to size×size pixels).
func WithSize(size int) Option {
	return func(o *Options) { o.Size = size }
}

// WithFormat sets the desired output format.
func WithFormat(format TargetFormat) Option {
	return func(o *Options) { o.Format = format }
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(o *Options) { o.HTTPClient = client }
}

// WithPreferredFormats sets the preferred favicon formats in priority order.
// Example: WithPreferredFormats(FormatSVG, FormatPNG) prefers SVG favicons
// and falls back to PNG if no SVG is found.
func WithPreferredFormats(formats ...DetectedFormat) Option {
	return func(o *Options) {
		o.PreferredFormats = formats
	}
}

// httpClient returns an *http.Client based on the options.
func (o *Options) httpClient() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	return &http.Client{
		Timeout: o.RequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= o.MaxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}
