package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/uptime-app/backend/internal/server"
)

func TestCORSAndContentType(t *testing.T) {
	mux := http.NewServeMux()
	(&Controller{}).RegisterRoutes(mux)
	h := server.Middleware(mux, "http://localhost:3000")
	req := httptest.NewRequest("OPTIONS", "/auth/refresh", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("preflight: %d %v", w.Code, w.Header())
	}
	req = httptest.NewRequest("POST", "/auth/refresh", nil)
	req.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("origin status: %d", w.Code)
	}
	req = httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 415 {
		t.Fatalf("content type status: %d", w.Code)
	}
	req = httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{"email":"x","password":"y","extra":1}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("invalid JSON status: %d", w.Code)
	}
}
func TestCookie(t *testing.T) {
	a := Controller{CookieSecure: true}
	w := httptest.NewRecorder()
	a.respond(w, Tokens{Access: "access", Refresh: "refresh", AccessExpiresAt: time.Now().Add(time.Minute), RefreshExpiresAt: time.Now().Add(time.Hour)}, 200)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != "/auth/refresh" {
		t.Fatalf("cookie: %+v", cookies)
	}
	if strings.Contains(w.Body.String(), "refresh") {
		t.Fatal("refresh token leaked into JSON")
	}
}
