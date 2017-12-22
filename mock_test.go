package couchdb

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-kivik/couchdb/chttp"
	"github.com/go-kivik/kiviktest/kt"
	"github.com/pkg/errors"
)

type customTransport func(*http.Request) (*http.Response, error)

var _ http.RoundTripper = customTransport(nil)

func (t customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t(req)
}

func newTestDB(response *http.Response, err error) *db {
	return &db{
		dbName: "testdb",
		client: newTestClient(response, err),
	}
}

func newCustomDB(fn func(*http.Request) (*http.Response, error)) *db {
	return &db{
		dbName: "testdb",
		client: newCustomClient(fn),
	}
}

func newTestClient(response *http.Response, err error) *client {
	return newCustomClient(func(req *http.Request) (*http.Response, error) {
		if e := consume(req.Body); e != nil {
			return nil, e
		}
		if err != nil {
			return nil, err
		}
		response := response
		response.Request = req
		return response, nil
	})
}

func newCustomClient(fn func(*http.Request) (*http.Response, error)) *client {
	chttpClient, _ := chttp.New(context.Background(), "http://example.com/")
	chttpClient.Client.Transport = customTransport(fn)
	return &client{
		Client: chttpClient,
	}
}

func Body(str string) io.ReadCloser {
	if !strings.HasSuffix(str, "\n") {
		str = str + "\n"
	}
	return ioutil.NopCloser(strings.NewReader(str))
}

func parseTime(t *testing.T, str string) time.Time {
	ts, err := time.Parse(time.RFC3339, str)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// consume consumes and closes r or does nothing if it is nil.
func consume(r io.ReadCloser) error {
	if r == nil {
		return nil
	}
	defer r.Close() // nolint: errcheck
	_, e := ioutil.ReadAll(r)
	return e
}

func realDB(t *testing.T) *db {
	driver := &Couch{}
	c, err := driver.NewClient(context.Background(), kt.DSN(t))
	if err != nil {
		if _, ok := errors.Cause(err).(*url.Error); ok {
			t.Skip("Cannot connect to CouchDB")
		}
		t.Fatal(err)
	}
	dbname := kt.TestDBName(t)

	if err := c.CreateDB(context.Background(), dbname, nil); err != nil {
		t.Fatal(err)
	}
	return &db{
		client: c.(*client),
		dbName: dbname,
	}
}
