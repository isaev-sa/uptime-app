package auth

import (
	"net/http"
	"time"
)

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type tokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatar_url"`
}
type profileResponse struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}
type profileRequest struct {
	Name string `json:"name"`
}
type monitorRequest struct {
	URL             string `json:"url"`
	IntervalSeconds int    `json:"interval_seconds"`
}
type fileResponse struct {
	URL string `json:"url"`
}
type avatarResponse struct {
	AvatarURL string `json:"avatar_url"`
}

// @Summary List monitors
// @Description Pass before and before_id together, using the last item from the previous page.
// @Tags monitors
// @Produce json
// @Security BearerAuth
// @Param before query string false "Cursor timestamp (RFC3339)"
// @Param before_id query string false "Cursor monitor UUID"
// @Success 200 {object} monitorPage
// @Failure 400 {string} string "Invalid cursor"
// @Failure 401 {string} string "Invalid credentials"
// @Failure 500 {string} string "Internal server error"
// @Router /monitors [get]
func (a *Controller) listMonitors(w http.ResponseWriter, r *http.Request) { a.monitors(w, r) }

// @Summary Create a monitor
// @Tags monitors
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body monitorRequest true "HTTP or HTTPS URL and interval in seconds"
// @Success 201 {object} monitor
// @Failure 400 {string} string "Invalid monitor"
// @Failure 401 {string} string "Invalid credentials"
// @Failure 409 {string} string "Monitor limit reached"
// @Failure 500 {string} string "Internal server error"
// @Router /monitors [post]
func (a *Controller) createMonitor(w http.ResponseWriter, r *http.Request) { a.monitors(w, r) }

// @Summary Get the current profile
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} profileResponse
// @Failure 401 {string} string "Invalid credentials"
// @Failure 500 {string} string "Internal server error"
// @Router /profile [get]
func (a *Controller) getProfile(w http.ResponseWriter, r *http.Request) { a.profile(w, r) }

// @Summary Update the current profile
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body profileRequest true "New profile name"
// @Success 200 {object} profileResponse
// @Failure 400 {string} string "Invalid request"
// @Failure 401 {string} string "Invalid credentials"
// @Failure 500 {string} string "Internal server error"
// @Router /profile [patch]
func (a *Controller) updateProfile(w http.ResponseWriter, r *http.Request) { a.profile(w, r) }
