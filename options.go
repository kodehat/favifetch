package favifetch

import (
	"net/http"
	"time"
)

// Options holds all configuration for the favicon fetcher.
type Options struct {
	// UserAgent sent in HTTP requests for HTML and manifest fetching.
	// For actual favicon image fetches, both this and a browser-like UA are tried.
	UserAgent string

	// RequestTimeout is the maximum time for the entire fetch operation.
	RequestTimeout time.Duration

	// MaxImageSize is the maximum size (in bytes) of a favicon image to accept.
	MaxImageSize int64

	// MaxRedirects limits the number of HTTP redirects to follow.
	MaxRedirects int

	// BlockPrivateIps controls whether requests to private IP ranges are rejected.
	BlockPrivateIps bool

	// UseFallbackAPI enables the Google s2/favicons API as a last-resort fallback.
	UseFallbackAPI bool

	// Size is the desired output size in pixels (resizes favicon to size×size).
	// 0 means no resizing.
	Size int

	// Format is the desired output format. Zero value (TargetUnspecified) means keep original.
	Format TargetFormat

	// HTTPClient is the *http.Client to use for all requests. If nil, a default
	// client is created from the other options.
	HTTPClient *http.Client
}

// DefaultOptions returns the default configuration.
// Optional Option arguments can be passed to override defaults.
func DefaultOptions(opts ...Option) *Options {
	o := &Options{
		UserAgent:       "Favifetch/1.0",
		RequestTimeout:  5 * time.Second,
		MaxImageSize:    5 * 1024 * 1024, // 5MB
		MaxRedirects:    5,
		BlockPrivateIps: true,
		UseFallbackAPI:  true,
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

// WithUserAgent sets the User-Agent header.
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

// WithFallbackAPI enables or disables the Google favicon API fallback.
func WithFallbackAPI(use bool) Option {
	return func(o *Options) { o.UseFallbackAPI = use }
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
