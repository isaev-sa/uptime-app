package auth

import (
	"context"
	"errors"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"example.com/uptime-app/backend/internal/storage"
	"github.com/google/uuid"
)

func TestPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := storage.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	schema := "test_" + uuid.NewString()
	schema = "test_" + stringsWithoutDashes(schema[5:])
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
	if err = storage.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	s := Service{DB: db, Secret: []byte("01234567890123456789012345678901"), AccessTTL: time.Minute, RefreshTTL: time.Hour}
	ctx := context.Background()
	tokens, err := s.Register(ctx, "Имя", "User@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Register(ctx, "Имя", "user@example.com", "password123"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err = s.Login(ctx, "user@example.com", "badpassword"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("bad login: %v", err)
	}
	if _, err = s.Login(ctx, "user@example.com", "password123"); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Refresh(ctx, tokens.Refresh); err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrRefresh) {
				t.Errorf("refresh: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful refreshes: %d", successes.Load())
	}
	if _, err = s.Refresh(ctx, tokens.Refresh); !errors.Is(err, ErrRefresh) {
		t.Fatalf("replay: %v", err)
	}
	if _, err = s.Refresh(ctx, "invalid"); !errors.Is(err, ErrRefresh) {
		t.Fatalf("invalid: %v", err)
	}
	expiredService := s
	expiredService.RefreshTTL = -time.Second
	expired, err := expiredService.Login(ctx, "user@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Refresh(ctx, expired.Refresh); !errors.Is(err, ErrRefresh) {
		t.Fatalf("expired: %v", err)
	}
}
func stringsWithoutDashes(s string) string {
	b := make([]byte, 0, len(s))
	for i := range s {
		if s[i] != '-' {
			b = append(b, s[i])
		}
	}
	return string(b)
}
