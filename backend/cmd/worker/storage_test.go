package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestMinutePersistence(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	// Temporary relations isolate batch replay and retention from any live monitors.
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TEMP TABLE monitors(id uuid PRIMARY KEY,url_version bigint)")
	exec("CREATE TEMP TABLE monitor_minutes(monitor_id uuid,minute timestamptz,successes int,failures int,last_batch uuid,PRIMARY KEY(monitor_id,minute))")
	id := uuid.NewString()
	batch := uuid.NewString()
	exec("INSERT INTO monitors VALUES ($1,2)", id)
	now := time.Now().UTC().Truncate(time.Minute)
	exec(saveSQL, id, 2, now, 11, 1, batch)
	exec(saveSQL, id, 2, now, 11, 1, batch)
	exec(saveSQL, id, 1, now, 0, 20, uuid.NewString())
	exec(saveSQL, id, 2, now.Add(-31*24*time.Hour), 1, 0, uuid.NewString())
	exec(saveSQL, id, 2, now.Add(-29*24*time.Hour), 1, 0, uuid.NewString())
	exec(expirySQL)
	var rows, good, bad int
	if err := conn.QueryRow(ctx, "SELECT count(*),sum(successes),sum(failures) FROM monitor_minutes").Scan(&rows, &good, &bad); err != nil {
		t.Fatal(err)
	}
	if rows != 2 || good != 12 || bad != 1 {
		t.Fatalf("rows=%d successes=%d failures=%d", rows, good, bad)
	}
}
