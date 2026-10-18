package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func Router(log *slog.Logger, token, allowedOrigin string, ping func(context.Context) error, registerModules func(chi.Router)) http.Handler {
	r := chi.NewRouter()
	r.Use(requestLog(log), recoverPanic(log))
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "unhealthy", "database": "unavailable",
				"error": map[string]string{"code": "database_unavailable", "message": "database unavailable"},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authenticate(token, allowedOrigin))
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
				next.ServeHTTP(w, req)
			})
		})
		r.Get("/system/info", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"name": "DevOrbit", "api_version": "v1"})
		})
		if registerModules != nil {
			registerModules(r)
		}
	})
	return r
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func authenticate(token, allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origins := r.Header.Values("Origin"); len(origins) > 1 || len(origins) == 1 && origins[0] != allowedOrigin {
				writeError(w, http.StatusForbidden, "origin_forbidden", "origin is not allowed")
				return
			}
			credentials := r.Header.Values("Authorization")
			if len(credentials) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
				return
			}
			provided, ok := strings.CutPrefix(credentials[0], "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wrapped, r)
			log.Info("http request", "method", r.Method, "route", chi.RouteContext(r.Context()).RoutePattern(), "status", wrapped.Status(), "duration", time.Since(start))
		})
	}
}

func recoverPanic(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recover() != nil {
					log.Error("request panic")
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
