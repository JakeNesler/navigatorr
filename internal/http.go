package internal

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SameOriginRedirect prevents authenticated requests and their Referer headers
// from reaching another origin, including a different port or scheme.
func SameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if len(via) == 0 {
		return nil
	}
	first := via[0].URL
	if !strings.EqualFold(first.Scheme, req.URL.Scheme) ||
		!strings.EqualFold(first.Hostname(), req.URL.Hostname()) ||
		originPort(first) != originPort(req.URL) {
		return errors.New("cross-origin redirects are disabled")
	}
	return nil
}

func originPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

// HTTPError retains the cause without the request URL, which may contain auth
// credentials in its query string. The cause remains available to errors.Is/As.
func HTTPError(stage string, err error) error {
	for {
		var uerr *url.Error
		if !errors.As(err, &uerr) || uerr.Err == nil {
			return fmt.Errorf("%s: %w", stage, err)
		}
		err = uerr.Err
	}
}
