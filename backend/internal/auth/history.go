package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type historyBucket struct {
	MonitorID string    `json:"monitor_id"`
	Start     time.Time `json:"start"`
	Successes int64     `json:"successes"`
	Failures  int64     `json:"failures"`
}
type historyPage struct {
	Start       time.Time       `json:"start"`
	StepSeconds int             `json:"step_seconds"`
	Count       int             `json:"count"`
	Buckets     []historyBucket `json:"buckets"`
}

// @Summary Get availability history for the current user's monitors
// @Description Returns UTC buckets with observations. Missing buckets have no data.
// @Tags monitors
// @Security BearerAuth
// @Produce json
// @Param ids query string true "Comma-separated monitor UUIDs (1 to 50)"
// @Param period query string true "History period" Enums(1h,24h,30d)
// @Success 200 {object} historyPage
// @Failure 400 {string} string "Invalid period or IDs"
// @Failure 401 {string} string "Invalid credentials"
// @Failure 500 {string} string "Internal server error"
// @Router /monitors/history [get]
func (a *Controller) history(w http.ResponseWriter, r *http.Request) {
	user, err := a.authenticate(r)
	if err != nil {
		respondError(w, err)
		return
	}
	q := r.URL.Query()
	ids := strings.Split(q.Get("ids"), ",")
	step := map[string]int{"1h": 60, "24h": 900, "30d": 21600}[q.Get("period")]
	count := map[string]int{"1h": 60, "24h": 96, "30d": 120}[q.Get("period")]
	if step == 0 || len(ids) > monitorPageSize {
		http.Error(w, "invalid history query", 400)
		return
	}
	for i, id := range ids {
		v, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "invalid monitor ID", 400)
			return
		}
		ids[i] = v.String()
	}
	now := time.Now().UTC().Truncate(time.Minute)
	width := time.Duration(step) * time.Second
	start := now.Add(-time.Duration(count) * width)
	page := historyPage{Start: start, StepSeconds: step, Count: count, Buckets: []historyBucket{}}
	err = a.Service.DB.WithContext(r.Context()).Raw(`SELECT h.monitor_id,
 date_bin(make_interval(secs => ?), h.minute, ?::timestamptz) AS start,
 sum(h.successes) AS successes,sum(h.failures) AS failures
 FROM monitor_minutes h JOIN monitors m ON h.monitor_id=m.id
 WHERE m.user_id=? AND m.id IN ? AND h.minute>=? AND h.minute<?
 GROUP BY h.monitor_id,start ORDER BY h.monitor_id,start`, step, start, user, ids, start, now).Scan(&page.Buckets).Error
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(page)
}
