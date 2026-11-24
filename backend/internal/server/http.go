package server

import (
	"mime"
	"net/http"
)

// Middleware returns a handler that enforces the allowed origin and request
// content types, answers preflight requests, and delegates other requests to next.
func Middleware(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin != "" && origin != allowedOrigin {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || (media != "application/json" && !(r.Method == http.MethodPost && (r.URL.Path == "/profile/avatar" || r.URL.Path == "/files") && media == "multipart/form-data")) {
				http.Error(w, "application/json required", http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
