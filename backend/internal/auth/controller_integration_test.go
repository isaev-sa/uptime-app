package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"example.com/uptime-app/backend/internal/server"
	"example.com/uptime-app/backend/internal/storage"
	"github.com/google/uuid"
)

func TestAuthenticationHTTPWithPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := storage.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	adminPool, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = admin.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA "` + schema + `" CASCADE`)
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := storage.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = storage.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	service := &Service{DB: db, Secret: []byte("01234567890123456789012345678901"), AccessTTL: time.Minute, RefreshTTL: time.Hour}
	mux := http.NewServeMux()
	(&Controller{Service: service, AvatarDir: t.TempDir()}).RegisterRoutes(mux)
	handler := server.Middleware(mux, "http://localhost:3000")
	call := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:3000")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	data := `{"name":"  Анна  ","email":"test@example.com","password":"password123"}`
	loginData := `{"email":"test@example.com","password":"password123"}`
	if got := call("/auth/register", `{"name":"   ","email":"invalid@example.com","password":"password123"}`, nil).Code; got != 400 {
		t.Fatalf("empty name: %d", got)
	}
	registered := call("/auth/register", data, nil)
	if registered.Code != 201 {
		t.Fatalf("register: %d %s", registered.Code, registered.Body)
	}
	var response struct {
		Access string `json:"access_token"`
		Email  string `json:"email"`
		Name   string `json:"name"`
	}
	if err = json.Unmarshal(registered.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if _, err = service.VerifyAccess(response.Access); err != nil {
		t.Fatal(err)
	}
	if response.Name != "Анна" {
		t.Fatalf("name: %q", response.Name)
	}
	if response.Email != "test@example.com" {
		t.Fatalf("email: %q", response.Email)
	}
	monitorCall := func(method, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/monitors", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if got := monitorCall(http.MethodGet, "", "invalid").Code; got != http.StatusUnauthorized {
		t.Fatalf("monitor auth: %d", got)
	}
	if got := monitorCall(http.MethodPost, `{"url":"https://example.com","interval_seconds":0}`, response.Access).Code; got != http.StatusBadRequest {
		t.Fatalf("invalid interval: %d", got)
	}
	created := monitorCall(http.MethodPost, `{"url":"https://example.com/path","interval_seconds":300}`, response.Access)
	if created.Code != http.StatusCreated {
		t.Fatalf("create monitor: %d %s", created.Code, created.Body)
	}
	listed := monitorCall(http.MethodGet, "", response.Access)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), "https://example.com/path") || !strings.Contains(listed.Body.String(), `"interval_seconds":300`) {
		t.Fatalf("list monitors: %d %s", listed.Code, listed.Body)
	}
	other := call("/auth/register", `{"name":"Other","email":"other@example.com","password":"password123"}`, nil)
	var otherResponse struct {
		Access string `json:"access_token"`
	}
	if err := json.Unmarshal(other.Body.Bytes(), &otherResponse); err != nil || other.Code != http.StatusCreated {
		t.Fatalf("other user: %d %s", other.Code, other.Body)
	}
	if got := monitorCall(http.MethodGet, "", otherResponse.Access); got.Code != http.StatusOK || strings.TrimSpace(got.Body.String()) != `{"items":[],"has_more":false}` {
		t.Fatalf("monitor isolation: %d %s", got.Code, got.Body)
	}

	var createdMonitor monitor
	if err := json.Unmarshal(created.Body.Bytes(), &createdMonitor); err != nil {
		t.Fatal(err)
	}
	monitorByID := func(method, id, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/monitors/"+id, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		if method == http.MethodPatch {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	readHistory := func(token string) historyPage {
		t.Helper()
		w := monitorByID(http.MethodGet, "history?period=1h&ids="+createdMonitor.ID, "", token)
		var history historyPage
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil {
			t.Fatalf("history: %d %s", w.Code, w.Body)
		}
		return history
	}
	if err := db.Exec(`INSERT INTO monitor_minutes VALUES (?,date_trunc('minute',now()) - interval '1 minute',11,1,?)`, createdMonitor.ID, uuid.NewString()).Error; err != nil {
		t.Fatal(err)
	}
	monitorByID(http.MethodPatch, createdMonitor.ID, `{"url":"https://example.com/path","interval_seconds":30}`, response.Access)
	history := readHistory(response.Access)
	if len(history.Buckets) != 1 || history.Buckets[0].Successes != 11 || history.Buckets[0].Failures != 1 || history.Count != 60 {
		t.Fatalf("owner history: %+v", history)
	}
	if len(readHistory(otherResponse.Access).Buckets) != 0 {
		t.Fatal("history leaked to another user")
	}
	validUpdate := `{"url":"https://updated.example.com","interval_seconds":120}`
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		if got := monitorByID(method, createdMonitor.ID, validUpdate, "invalid"); got.Code != http.StatusUnauthorized {
			t.Fatalf("%s auth: %d", method, got.Code)
		}
		if got := monitorByID(method, "bad-id", validUpdate, response.Access); got.Code != http.StatusBadRequest {
			t.Fatalf("%s invalid id: %d", method, got.Code)
		}
		if got := monitorByID(method, createdMonitor.ID, validUpdate, otherResponse.Access); got.Code != http.StatusNotFound {
			t.Fatalf("%s other user: %d", method, got.Code)
		}
		if got := monitorByID(method, uuid.NewString(), validUpdate, response.Access); got.Code != http.StatusNotFound {
			t.Fatalf("%s missing: %d", method, got.Code)
		}
	}
	if got := monitorByID(http.MethodPatch, createdMonitor.ID, `{"url":"bad","interval_seconds":0}`, response.Access); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid update: %d", got.Code)
	}
	updatedSite := monitorByID(http.MethodPatch, createdMonitor.ID, validUpdate, response.Access)
	if len(readHistory(response.Access).Buckets) != 0 {
		t.Fatal("old URL history survived edit")
	}
	var updatedMonitor monitor
	if err := json.Unmarshal(updatedSite.Body.Bytes(), &updatedMonitor); err != nil || updatedSite.Code != http.StatusOK || updatedMonitor.ID != createdMonitor.ID || updatedMonitor.URL != "https://updated.example.com" || updatedMonitor.IntervalSeconds != 120 || !updatedMonitor.CreatedAt.Equal(createdMonitor.CreatedAt) {
		t.Fatalf("update: %d %s", updatedSite.Code, updatedSite.Body)
	}
	if got := monitorCall(http.MethodGet, "", response.Access); !strings.Contains(got.Body.String(), "https://updated.example.com") {
		t.Fatalf("updated list: %s", got.Body)
	}
	if got := monitorByID(http.MethodDelete, createdMonitor.ID, "", response.Access); got.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", got.Code, got.Body)
	}
	if got := monitorCall(http.MethodGet, "", response.Access); strings.Contains(got.Body.String(), createdMonitor.ID) {
		t.Fatalf("deleted monitor still listed: %s", got.Body)
	}
	preflight := httptest.NewRequest(http.MethodOptions, "/monitors/"+createdMonitor.ID, nil)
	preflight.Header.Set("Origin", "http://localhost:3000")
	preflightResult := httptest.NewRecorder()
	handler.ServeHTTP(preflightResult, preflight)
	if !strings.Contains(preflightResult.Header().Get("Access-Control-Allow-Methods"), "DELETE") {
		t.Fatal("DELETE missing from CORS methods")
	}

	otherID, err := service.VerifyAccess(otherResponse.Access)
	if err != nil {
		t.Fatal(err)
	}
	seed := make([]monitor, maxMonitors-1)
	for i := range seed {
		seed[i] = monitor{ID: uuid.NewString(), UserID: otherID, URL: "https://example.com", IntervalSeconds: 60}
	}
	if err := db.CreateInBatches(seed, 100).Error; err != nil {
		t.Fatal(err)
	}
	if got := monitorCall(http.MethodPost, `{"url":"https://example.com","interval_seconds":60}`, otherResponse.Access); got.Code != http.StatusCreated {
		t.Fatalf("create at limit: %d %s", got.Code, got.Body)
	}
	if got := monitorCall(http.MethodPost, `{"url":"https://example.com","interval_seconds":60}`, otherResponse.Access); got.Code != http.StatusConflict {
		t.Fatalf("create over limit: %d %s", got.Code, got.Body)
	}
	first := monitorCall(http.MethodGet, "", otherResponse.Access)
	var page monitorPage
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil || len(page.Items) != monitorPageSize || !page.HasMore {
		t.Fatalf("first page: %d %s", first.Code, first.Body)
	}
	last := page.Items[len(page.Items)-1]
	query := url.Values{"before": {last.CreatedAt.Format(time.RFC3339Nano)}, "before_id": {last.ID}}
	req := httptest.NewRequest(http.MethodGet, "/monitors?"+query.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+otherResponse.Access)
	next := httptest.NewRecorder()
	handler.ServeHTTP(next, req)
	var nextPage monitorPage
	if err := json.Unmarshal(next.Body.Bytes(), &nextPage); err != nil || len(nextPage.Items) != monitorPageSize || !nextPage.HasMore {
		t.Fatalf("second page: %d %s", next.Code, next.Body)
	}
	for _, item := range nextPage.Items {
		for _, previous := range page.Items {
			if item.ID == previous.ID {
				t.Fatalf("duplicate monitor across pages: %s", item.ID)
			}
		}
	}
	profileCall := func(method, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/profile", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		if method == http.MethodPatch {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if got := profileCall(http.MethodGet, "", "invalid").Code; got != http.StatusUnauthorized {
		t.Fatalf("invalid token: %d", got)
	}
	if got := profileCall(http.MethodPatch, `{"name":"   "}`, response.Access).Code; got != http.StatusBadRequest {
		t.Fatalf("invalid name: %d", got)
	}
	updated := profileCall(http.MethodPatch, `{"name":"Мария"}`, response.Access)
	if updated.Code != http.StatusOK {
		t.Fatalf("update profile: %d %s", updated.Code, updated.Body)
	}
	profile := profileCall(http.MethodGet, "", response.Access)
	if profile.Code != http.StatusOK || !strings.Contains(profile.Body.String(), "Мария") || strings.Contains(profile.Body.String(), "password") {
		t.Fatalf("profile: %d %s", profile.Code, profile.Body)
	}
	updated = profileCall(http.MethodPatch, `{"name":" Мария Иванова "}`, response.Access)
	if updated.Code != http.StatusOK {
		t.Fatalf("name with space: %d %s", updated.Code, updated.Body)
	}
	profile = profileCall(http.MethodGet, "", response.Access)
	var saved struct {
		Name string `json:"name"`
	}
	if err = json.Unmarshal(profile.Body.Bytes(), &saved); err != nil || saved.Name != "Мария Иванова" {
		t.Fatalf("name with space not saved: %s, %v", profile.Body, err)
	}

	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	upload := func(token string, data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("avatar", "avatar.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(data); err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/profile/avatar", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if got := upload("invalid", imageData.Bytes()).Code; got != http.StatusUnauthorized {
		t.Fatalf("upload auth: %d", got)
	}
	if got := upload(response.Access, []byte("not an image")).Code; got != http.StatusBadRequest {
		t.Fatalf("invalid image: %d", got)
	}
	uploaded := upload(response.Access, imageData.Bytes())
	var avatar struct {
		URL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(uploaded.Body.Bytes(), &avatar); err != nil || uploaded.Code != http.StatusOK || avatar.URL == "" {
		t.Fatalf("upload: %d %s %v", uploaded.Code, uploaded.Body, err)
	}
	imageResponse := httptest.NewRecorder()
	handler.ServeHTTP(imageResponse, httptest.NewRequest(http.MethodGet, avatar.URL, nil))
	if imageResponse.Code != http.StatusOK || !bytes.Equal(imageResponse.Body.Bytes(), imageData.Bytes()) {
		t.Fatalf("served avatar: %d", imageResponse.Code)
	}
	if p := profileCall(http.MethodGet, "", response.Access); !strings.Contains(p.Body.String(), avatar.URL) {
		t.Fatalf("profile avatar: %s", p.Body)
	}
	cookie := registered.Result().Cookies()[0]
	if got := call("/auth/register", data, nil).Code; got != 409 {
		t.Fatalf("duplicate: %d", got)
	}
	if got := call("/auth/login", `{"email":"test@example.com","password":"wrongpass"}`, nil).Code; got != 401 {
		t.Fatalf("wrong password: %d", got)
	}
	login := call("/auth/login", loginData, nil)
	if login.Code != 200 {
		t.Fatalf("login: %d", login.Code)
	}
	if err = json.Unmarshal(login.Body.Bytes(), &response); err != nil || response.Name != "Мария Иванова" {
		t.Fatalf("login name: %q, %v", response.Name, err)
	}
	refreshed := call("/auth/refresh", "{}", cookie)
	if refreshed.Code != 200 {
		t.Fatalf("refresh: %d %s", refreshed.Code, refreshed.Body)
	}
	if err = json.Unmarshal(refreshed.Body.Bytes(), &response); err != nil || response.Name != "Мария Иванова" {
		t.Fatalf("refresh name: %q, %v", response.Name, err)
	}
	if refreshed.Result().Cookies()[0].Value == cookie.Value {
		t.Fatal("refresh cookie was not rotated")
	}
	if got := call("/auth/refresh", "{}", cookie).Code; got != 401 {
		t.Fatalf("replay: %d", got)
	}
	current := refreshed.Result().Cookies()[0]
	logout := call("/auth/refresh/logout", "{}", current)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", logout.Code, logout.Body)
	}
	cleared := logout.Result().Cookies()[0]
	if cleared.MaxAge >= 0 || cleared.Path != current.Path || !cleared.HttpOnly {
		t.Fatalf("logout cookie: %+v", cleared)
	}
	if got := call("/auth/refresh", "{}", current).Code; got != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: %d", got)
	}
}
