package arrservice

import (
	"encoding/base64"
	"net/http"
)

// AuthStrategy applies authentication to an HTTP request.
type AuthStrategy interface {
	Apply(req *http.Request)
}

// HeaderAuth sends the API key via a header (e.g. X-Api-Key).
type HeaderAuth struct {
	Header string
	Key    string
}

func (a *HeaderAuth) Apply(req *http.Request) {
	req.Header.Set(a.Header, a.Key)
}

// QueryAuth sends the API key as a query parameter.
type QueryAuth struct {
	Param string
	Key   string
}

func (a *QueryAuth) Apply(req *http.Request) {
	q := req.URL.Query()
	q.Set(a.Param, a.Key)
	req.URL.RawQuery = q.Encode()
}

// BasicAuth uses HTTP basic authentication.
type BasicAuth struct {
	Username string
	Password string
}

func (a *BasicAuth) Apply(req *http.Request) {
	creds := base64.StdEncoding.EncodeToString([]byte(a.Username + ":" + a.Password))
	req.Header.Set("Authorization", "Basic "+creds)
}
