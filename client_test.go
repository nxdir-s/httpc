package httpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"testing"

	_ "embed"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/response.json
var testData []byte

const (
	TestEndpoint  string = "/resource"
	TestResponse  string = "{}"
	TestRateLimit int    = 60
)

func TestGet(t *testing.T) {
	cases := []struct {
		endpoint     string
		headers      map[string]string
		expectedCode int
		expectedErr  error
	}{
		{
			endpoint:     TestEndpoint,
			headers:      nil,
			expectedCode: http.StatusOK,
			expectedErr:  nil,
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	cfg := &Config{
		BaseUrl:      ts.URL,
		TlsConfig:    &tls.Config{},
		RetryEnabled: true,
		OTelEnabled:  true,
	}

	client, err := NewClient(ctx, cfg,
		WithCustomClient(ts.Client()),
		WithDefaultHeaders(map[string]string{
			"Cache-Control": "no-cache",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {

			resp, err := client.Get(ctx, tt.endpoint, tt.headers)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)
		})
	}
}

func TestPost(t *testing.T) {
	cases := []struct {
		endpoint     string
		headers      map[string]string
		body         io.Reader
		expectedCode int
		expectedErr  error
	}{
		{
			endpoint:     TestEndpoint,
			headers:      nil,
			body:         bytes.NewReader([]byte(`{}`)),
			expectedCode: http.StatusOK,
			expectedErr:  nil,
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	cfg := &Config{
		BaseUrl:   ts.URL,
		TlsConfig: &tls.Config{},
	}

	client, err := NewClient(ctx, cfg,
		WithCustomClient(ts.Client()),
		WithDefaultHeaders(map[string]string{
			"Cache-Control": "no-cache",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	type Response struct {
		Data []struct {
			Key string `json:"key"`
		} `json:"data"`
	}

	var in Response
	if err := json.NewDecoder(bytes.NewReader(testData)).Decode(&in); err != nil {
		t.Fatal(err)
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {

			var out Response
			resp, err := client.Post(ctx, tt.endpoint, tt.body, tt.headers, &out)

			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)

			assert.True(t, reflect.DeepEqual(in, out))
		})
	}
}

func TestPut(t *testing.T) {
	cases := []struct {
		endpoint     string
		headers      map[string]string
		body         io.Reader
		expectedCode int
		expectedErr  error
	}{
		{
			endpoint:     TestEndpoint,
			headers:      nil,
			body:         bytes.NewReader([]byte(`{}`)),
			expectedCode: http.StatusOK,
			expectedErr:  nil,
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	cfg := &Config{
		BaseUrl:   ts.URL,
		TlsConfig: &tls.Config{},
	}

	client, err := NewClient(ctx, cfg,
		WithCustomClient(ts.Client()),
		WithDefaultHeaders(map[string]string{
			"Cache-Control": "no-cache",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	type Response struct {
		Data []struct {
			Key string `json:"key"`
		} `json:"data"`
	}

	var in Response
	if err := json.NewDecoder(bytes.NewReader(testData)).Decode(&in); err != nil {
		t.Fatal(err)
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {

			var out Response
			resp, err := client.Put(ctx, tt.endpoint, tt.body, tt.headers, &out)

			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)

			assert.True(t, reflect.DeepEqual(in, out))
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		endpoint     string
		headers      map[string]string
		body         io.Reader
		expectedCode int
		expectedErr  error
	}{
		{
			endpoint:     TestEndpoint,
			headers:      nil,
			body:         bytes.NewReader([]byte(`{}`)),
			expectedCode: http.StatusOK,
			expectedErr:  nil,
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	cfg := &Config{
		BaseUrl:   ts.URL,
		TlsConfig: &tls.Config{},
	}

	client, err := NewClient(ctx, cfg,
		WithCustomClient(ts.Client()),
		WithDefaultHeaders(map[string]string{
			"Cache-Control": "no-cache",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	type Response struct {
		Data []struct {
			Key string `json:"key"`
		} `json:"data"`
	}

	var in Response
	if err := json.NewDecoder(bytes.NewReader(testData)).Decode(&in); err != nil {
		t.Fatal(err)
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {

			var out Response
			resp, err := client.Delete(ctx, tt.endpoint, tt.body, tt.headers, &out)

			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)

			assert.True(t, reflect.DeepEqual(in, out))
		})
	}
}

func TestPatch(t *testing.T) {
	cases := []struct {
		endpoint     string
		headers      map[string]string
		body         io.Reader
		expectedCode int
		expectedErr  error
	}{
		{
			endpoint:     TestEndpoint,
			headers:      nil,
			body:         bytes.NewReader([]byte(`{}`)),
			expectedCode: http.StatusOK,
			expectedErr:  nil,
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	cfg := &Config{
		BaseUrl:   ts.URL,
		TlsConfig: &tls.Config{},
	}

	client, err := NewClient(ctx, cfg,
		WithCustomClient(ts.Client()),
		WithDefaultHeaders(map[string]string{
			"Cache-Control": "no-cache",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	type Response struct {
		Data []struct {
			Key string `json:"key"`
		} `json:"data"`
	}

	var in Response
	if err := json.NewDecoder(bytes.NewReader(testData)).Decode(&in); err != nil {
		t.Fatal(err)
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {

			var out Response
			resp, err := client.Patch(ctx, tt.endpoint, tt.body, tt.headers, &out)

			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)

			assert.True(t, reflect.DeepEqual(in, out))
		})
	}
}

func TestStream(t *testing.T) {
	cases := []struct {
		endpoint     string
		headers      map[string]string
		body         io.Reader
		expectedCode int
		expectedErr  error
	}{
		{
			endpoint:     TestEndpoint,
			headers:      nil,
			body:         bytes.NewReader([]byte(`{}`)),
			expectedCode: http.StatusOK,
			expectedErr:  nil,
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	cfg := &Config{
		BaseUrl:   ts.URL,
		TlsConfig: &tls.Config{},
	}

	client, err := NewClient(ctx, cfg,
		WithCustomClient(ts.Client()),
		WithDefaultHeaders(map[string]string{
			"Cache-Control": "no-cache",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	type Response struct {
		Data []struct {
			Key string `json:"key"`
		} `json:"data"`
	}

	var in Response
	if err := json.NewDecoder(bytes.NewReader(testData)).Decode(&in); err != nil {
		t.Fatal(err)
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {

			r, err := client.Stream(ctx, http.MethodGet, tt.endpoint, tt.body, tt.headers)

			var out Response
			if err := json.NewDecoder(r).Decode(&out); err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tt.expectedErr, err)
			assert.True(t, reflect.DeepEqual(in, out))
		})
	}
}

type ErrTest struct{}

func (e *ErrTest) Error() string {
	return "test error"
}

type ErrMissingMsg struct {
	errType string
}

func (e *ErrMissingMsg) Error() string {
	return "missing error message for " + e.errType
}

func TestErrors(t *testing.T) {
	var err error

	err = &ErrStatusCode{http.StatusInternalServerError, bytes.NewBufferString(TestResponse)}
	if len(err.Error()) == 0 {
		t.Error(&ErrMissingMsg{"ErrStatusCode"})
	}

	err = &ErrInvalidResource{&ErrTest{}}
	if len(err.Error()) == 0 {
		t.Error(&ErrMissingMsg{"ErrInvalidResource"})
	}

	err = &ErrDecode{&ErrTest{}}
	if len(err.Error()) == 0 {
		t.Error(&ErrMissingMsg{"ErrDecode"})
	}

	err = &ErrNewRequest{&ErrTest{}}
	if len(err.Error()) == 0 {
		t.Error(&ErrMissingMsg{"ErrNewRequest"})
	}

	err = &ErrRequest{&ErrTest{}}
	if len(err.Error()) == 0 {
		t.Error(&ErrMissingMsg{"ErrRequest"})
	}

	err = &ErrCopy{&ErrTest{}}
	if len(err.Error()) == 0 {
		t.Error(&ErrMissingMsg{"ErrCopy"})
	}
}
