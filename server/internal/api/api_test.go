package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func testRouter(ping func(context.Context) error) http.Handler {
	return Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "test-token", "http://127.0.0.1:7456", ping, nil)
}

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ping   func(context.Context) error
		status int
	}{
		{"healthy", func(context.Context) error { return nil }, http.StatusOK},
		{"unhealthy", func(context.Context) error { return errors.New("password=secret") }, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			testRouter(tc.ping).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			if w.Code != tc.status || w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("health response: %d %s", w.Code, w.Body.String())
			}
			if tc.status == http.StatusServiceUnavailable && strings.Contains(w.Body.String(), "secret") {
				t.Fatal("health response exposed database error")
			}
		})
	}
}

func TestSystemInfoAuthenticationAndOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, token, origin string
		status              int
	}{
		{"missing token", "", "", http.StatusUnauthorized},
		{"wrong token", "wrong", "", http.StatusUnauthorized},
		{"foreign origin", "test-token", "https://evil.example", http.StatusForbidden},
		{"same origin", "test-token", "http://127.0.0.1:7456", http.StatusOK},
		{"local client", "test-token", "", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			testRouter(func(context.Context) error { return nil }).ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
			if w.Code == http.StatusOK {
				var body map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["api_version"] != "v1" || len(body) != 2 {
					t.Fatalf("unexpected system info: %s (%v)", w.Body.String(), err)
				}
			}
		})
	}
}

func TestUnknownRouteUsesJSONError(t *testing.T) {
	w := httptest.NewRecorder()
	testRouter(func(context.Context) error { return nil }).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/unknown", nil))
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"code":"not_found"`) {
		t.Fatalf("unexpected error response: %d %s", w.Code, w.Body.String())
	}
}

func TestRejectsDuplicateOriginAndOversizedBody(t *testing.T) {
	h := Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Post("/size", func(w http.ResponseWriter, req *http.Request) {
			if _, err := io.Copy(io.Discard, req.Body); err != nil {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/size", strings.NewReader("x"))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Add("Origin", "http://127.0.0.1:7456")
	req.Header.Add("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("duplicate origins: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/size", strings.NewReader("x"))
	req.Header.Add("Authorization", "Bearer token")
	req.Header.Add("Authorization", "Bearer wrong")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("duplicate credentials: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/size", strings.NewReader(strings.Repeat("x", 1<<20+1)))
	req.Header.Set("Authorization", "Bearer token")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: %d", w.Code)
	}
}
