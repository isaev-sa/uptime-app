package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"example.com/uptime-app/backend/internal/storage"
)

type Controller struct {
	Service      *Service
	CookieSecure bool
	AvatarDir    string
	UploadDir    string
}
type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRoutes registers the authentication, profile, upload, and monitor
// endpoints on mux. It panics if a route conflicts with an existing registration.
func (a *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /monitors", a.monitors)
	mux.HandleFunc("POST /monitors", a.monitors)
	mux.HandleFunc("GET /profile", a.profile)
	mux.HandleFunc("POST /profile/avatar", a.uploadAvatar)
	mux.HandleFunc("POST /files", a.uploadFile)
	mux.HandleFunc("GET /files/{name}", a.serveFile)
	mux.HandleFunc("GET /avatars/{name}", a.serveAvatar)
	mux.HandleFunc("PATCH /profile", a.profile)
	mux.HandleFunc("POST /auth/register", a.register)
	mux.HandleFunc("POST /auth/login", a.login)
	mux.HandleFunc("POST /auth/refresh", a.refresh)
	mux.HandleFunc("POST /auth/refresh/logout", a.logout)
}
func (a *Controller) authenticate(r *http.Request) (string, error) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrCredentials
	}
	return a.Service.VerifyAccess(parts[1])
}

func (a *Controller) profile(w http.ResponseWriter, r *http.Request) {
	id, err := a.authenticate(r)
	if err != nil {
		respondError(w, err)
		return
	}
	var user storage.User
	if r.Method == http.MethodPatch {
		var input struct {
			Name string `json:"name"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		user, err = a.Service.UpdateProfile(r.Context(), id, input.Name)
	} else {
		user, err = a.Service.Profile(r.Context(), id)
	}
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}{user.Name, user.Email, user.AvatarURL})
}
func (a *Controller) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		if err := a.Service.Logout(r.Context(), cookie.Value); err != nil {
			respondError(w, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: "", Path: "/auth/refresh", HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
func parseCredentials(w http.ResponseWriter, r *http.Request) (credentials, error) {
	var c credentials
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return c, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return c, errors.New("invalid JSON")
	}
	return c, nil
}
func (a *Controller) register(w http.ResponseWriter, r *http.Request) {
	c, err := parseCredentials(w, r)
	if err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	t, err := a.Service.Register(r.Context(), c.Name, c.Email, c.Password)
	if err != nil {
		respondError(w, err)
		return
	}
	a.respond(w, t, http.StatusCreated)
}
func (a *Controller) login(w http.ResponseWriter, r *http.Request) {
	c, err := parseCredentials(w, r)
	if err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	t, err := a.Service.Login(r.Context(), c.Email, c.Password)
	if err != nil {
		respondError(w, err)
		return
	}
	a.respond(w, t, http.StatusOK)
}
func (a *Controller) refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh_token")
	if err != nil {
		respondError(w, ErrRefresh)
		return
	}
	t, err := a.Service.Refresh(r.Context(), c.Value)
	if err != nil {
		respondError(w, err)
		return
	}
	a.respond(w, t, http.StatusOK)
}
func (a *Controller) respond(w http.ResponseWriter, t Tokens, status int) {
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: t.Refresh, Path: "/auth/refresh", HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: t.RefreshExpiresAt, MaxAge: int(time.Until(t.RefreshExpiresAt).Seconds())})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		AccessToken string    `json:"access_token"`
		TokenType   string    `json:"token_type"`
		ExpiresAt   time.Time `json:"expires_at"`
		Email       string    `json:"email"`
		Name        string    `json:"name"`
		AvatarURL   string    `json:"avatar_url"`
	}{t.Access, "Bearer", t.AccessExpiresAt, t.Email, t.Name, t.AvatarURL})
}
func respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		http.Error(w, "invalid name, email or password", http.StatusBadRequest)
	case errors.Is(err, ErrDuplicate):
		http.Error(w, "email already registered", http.StatusConflict)
	case errors.Is(err, ErrCredentials), errors.Is(err, ErrRefresh):
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
	default:
		log.Printf("auth request failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
