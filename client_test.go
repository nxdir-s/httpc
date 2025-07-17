package httpc

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	TestEndpoint string = "/resource"
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
		BaseUrl:   ts.URL,
		TlsConfig: &tls.Config{},
	}

	client, err := NewClient(ctx, cfg)
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
