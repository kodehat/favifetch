package favifetch

import (
	"net"
	"net/url"
	"regexp"
	"strings"
)

// ErrPrivateIP is returned when a URL resolves to a private IP and BlockPrivateIps is enabled.
type ErrPrivateIP struct {
	Host string
	IP   string
}

func (e *ErrPrivateIP) Error() string {
	return "favifetch: access to private IP not allowed: " + e.Host + " (" + e.IP + ")"
}

var privateIPPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^10\.`),
	regexp.MustCompile(`^172\.(1[6-9]|2[0-9]|3[01])\.`),
	regexp.MustCompile(`^192\.168\.`),
	regexp.MustCompile(`^169\.254\.`),
	regexp.MustCompile(`^fc00:`),
	regexp.MustCompile(`^fe80:`),
}

var privateHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
}

// isPrivateIP checks whether an IP address is in a private range.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return false
}

// isPrivateHost checks whether a hostname resolves to a private IP.
func isPrivateHost(host string) (bool, string) {
	host = strings.ToLower(host)

	// Strip port
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	if privateHosts[host] {
		return true, host
	}

	// Try to parse as IP first
	ip := net.ParseIP(host)
	if ip != nil {
		if isPrivateIP(ip) {
			return true, ip.String()
		}
		return false, ""
	}

	// Resolve hostname - check the first non-error result
	ips, err := net.LookupIP(host)
	if err != nil {
		return false, ""
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return true, ip.String()
		}
	}
	return false, ""
}

// validateURL normalizes a URL and optionally checks for SSRF.
// It returns the normalized URL string, the parsed URL, and any error.
func validateURL(rawURL string, blockPrivateIps bool) (string, *url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil, errMissingURL
	}

	// Add scheme if missing
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", nil, errInvalidURL{rawURL: rawURL, reason: err.Error()}
	}

	// Validate scheme
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", nil, errInvalidURL{rawURL: rawURL, reason: "unsupported scheme: " + parsed.Scheme}
	}

	// Validate hostname
	if parsed.Host == "" {
		return "", nil, errInvalidURL{rawURL: rawURL, reason: "missing host"}
	}

	// SSRF protection
	if blockPrivateIps {
		if isPrivate, ip := isPrivateHost(parsed.Host); isPrivate {
			return "", nil, &ErrPrivateIP{Host: parsed.Host, IP: ip}
		}
	}

	return parsed.String(), parsed, nil
}

// isDataURL checks if a URL is a data: URI.
func isDataURL(u string) bool {
	return strings.HasPrefix(u, "data:")
}

// parseDataURL parses a data: URI and returns the decoded bytes and MIME type.
func parseDataURL(dataURL string) ([]byte, string, error) {
	// data:[<media type>][;base64],<data>
	if !strings.HasPrefix(dataURL, "data:") {
		return nil, "", nil
	}

	rest := dataURL[5:] // strip "data:"
	mimeType := "text/plain"
	isBase64 := false

	// Check for base64
	if idx := strings.Index(rest, ";base64,"); idx >= 0 {
		mimeType = rest[:idx]
		isBase64 = true
		rest = rest[idx+8:] // after ";base64,"
	} else if idx := strings.Index(rest, ","); idx >= 0 {
		if idx > 0 {
			mimeType = rest[:idx]
		}
		rest = rest[idx+1:]
	}

	if mimeType == "" {
		mimeType = "text/plain"
	}

	var data []byte
	var err error
	if isBase64 {
		data, err = base64Decode(rest)
	} else {
		decoded, uerr := url.PathUnescape(rest)
		if uerr != nil {
			decoded = rest
		}
		data = []byte(decoded)
	}
	return data, mimeType, err
}
