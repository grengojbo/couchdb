package chttp

import (
	"context"
	"net/http"
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"

	"github.com/go-kivik/kivik"
	"github.com/go-kivik/kivik/errors"
)

// Authenticator is an interface that provides authentication to a server.
type Authenticator interface {
	Authenticate(context.Context, *Client) error
}

// BasicAuth provides HTTP Basic Auth for a client.
type BasicAuth struct {
	Username string
	Password string

	// transport stores the original transport that is overridden by this auth
	// mechanism
	transport http.RoundTripper
}

var _ Authenticator = &BasicAuth{}

// RoundTrip fulfills the http.RoundTripper interface. It sets HTTP Basic Auth
// on outbound requests.
func (a *BasicAuth) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(a.Username, a.Password)
	transport := a.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(req)
}

// Authenticate sets HTTP Basic Auth headers for the client.
func (a *BasicAuth) Authenticate(ctx context.Context, c *Client) error {
	a.transport = c.Transport
	c.Transport = a
	return nil
}

// CookieAuth provides CouchDB Cookie auth services as described at
// http://docs.couchdb.org/en/2.0.0/api/server/authn.html#cookie-authentication
//
// CookieAuth stores authentication state after use, so should not be re-used.
type CookieAuth struct {
	Username string `json:"name"`
	Password string `json:"password"`

	client *Client
}

var _ Authenticator = &CookieAuth{}

// Authenticate initiates a session with the CouchDB server.
func (a *CookieAuth) Authenticate(ctx context.Context, c *Client) error {
	if err := a.setCookieJar(c); err != nil {
		return err // impossible error
	}
	a.client = c
	opts := &Options{
		Body: EncodeBody(a),
	}
	if _, err := c.DoError(ctx, kivik.MethodPost, "/_session", opts); err != nil {
		return err
	}
	return ValidateAuth(ctx, a.Username, c)
}

// Cookie returns the current session cookie if found, or nil if not.
func (a *CookieAuth) Cookie() *http.Cookie {
	if a.client == nil {
		return nil
	}
	for _, cookie := range a.client.Jar.Cookies(a.client.dsn) {
		if cookie.Name == kivik.SessionCookieName {
			return cookie
		}
	}
	return nil
}

// ValidateAuth validates that the requested username is authenticated.
func ValidateAuth(ctx context.Context, username string, client *Client) error {
	// This does a final request to validate that auth was successful. Cookies
	// may be filtered by a proxy, or a misconfigured client, so this check is
	// necessary.
	result := struct {
		Ctx struct {
			Name string `json:"name"`
		} `json:"userCtx"`
	}{}
	if _, err := client.DoJSON(ctx, "GET", "/_session", nil, &result); err != nil {
		return err
	}
	if result.Ctx.Name != username {
		return errors.Status(kivik.StatusBadResponse, "auth response for unexpected user")
	}
	return nil
}

func (a *CookieAuth) setCookieJar(c *Client) error {
	// If a jar is already set, just use it
	if c.Jar != nil {
		return nil
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return err // impossible error
	}
	c.Jar = jar
	return nil
}
