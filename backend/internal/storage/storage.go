package storage

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrations embed.FS

type User struct {
	ID           string `gorm:"type:uuid;primaryKey"`
	Email        string
	Name         string
	AvatarURL    string
	PasswordHash string
	CreatedAt    time.Time
}
type RefreshSession struct {
	ID        string `gorm:"type:uuid;primaryKey"`
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Open connects to PostgreSQL using dsn and returns the GORM database handle
// or a connection error.
func Open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
}

// Migrate applies pending embedded SQL migrations in filename order in one
// transaction. It returns filesystem or database errors; ctx controls the
// transaction and its queries. Concurrent migrations are serialized.
func Migrate(ctx context.Context, db *gorm.DB) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(753196284)).Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())").Error; err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
				continue
			}
			var count int64
			if err := tx.Table("schema_migrations").Where("version = ?", name).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				continue
			}
			data, err := migrations.ReadFile("migrations/" + name)
			if err != nil {
				return err
			}
			if err = tx.Exec(string(data)).Error; err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
			if err = tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
