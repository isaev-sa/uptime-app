package main

import (
	"context"
	"errors"

	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"example.com/uptime-app/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type target struct {
	id, address, host string
	interval          time.Duration
	revision, version int64
	next              time.Time
	cancel            context.CancelFunc
}
type result struct {
	target      *target
	ok          bool
	interrupted bool
	minute      time.Time
}
type bucket struct {
	id      string
	version int64
	minute  time.Time
}
type counts struct{ success, failure int }
type scheduler struct {
	conn            *pgx.Conn
	targets         map[string]*target
	hosts           map[string]int
	active, skipped int
	lag             time.Duration
	results         chan result
	minutes         map[bucket]counts
}

func (s *scheduler) reconcile(ctx context.Context) error {
	rows, err := s.conn.Query(ctx, "SELECT id::text,url,interval_seconds,revision,url_version FROM monitors")
	if err != nil {
		return err
	}
	defer rows.Close()
	current := map[string]*target{}
	for rows.Next() {
		var t target
		var seconds int
		if err := rows.Scan(&t.id, &t.address, &seconds, &t.revision, &t.version); err != nil {
			return err
		}
		if old := s.targets[t.id]; old != nil && old.revision == t.revision {
			current[t.id] = old
			continue
		}
		u, err := url.Parse(t.address)
		if err != nil {
			return err
		}
		t.host = strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		t.interval = time.Duration(seconds) * time.Second
		t.next = time.Now()
		current[t.id] = &t
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for id, t := range s.targets {
		if current[id] != t && t.cancel != nil {
			t.cancel()
		}
	}
	s.targets = current
	return nil
}

func (s *scheduler) dispatch(ctx context.Context, now time.Time) {
	for _, t := range s.targets {
		if now.Before(t.next) {
			continue
		}
		delay := now.Sub(t.next)
		s.lag = max(s.lag, delay)
		ticks := delay/t.interval + 1
		t.next = t.next.Add(ticks * t.interval)
		s.skipped += int(ticks - 1)
		if t.cancel != nil || s.active >= 64 || s.hosts[t.host] >= 4 {
			s.skipped++
			continue
		}
		check, cancel := context.WithCancel(ctx)
		t.cancel = cancel
		s.active++
		s.hosts[t.host]++
		go func() {
			ok := probe(check, t.address)
			s.results <- result{t, ok, check.Err() != nil, time.Now().UTC().Truncate(time.Minute)}
		}()
	}
}

func (s *scheduler) record(r result) {
	t := r.target
	t.cancel()
	t.cancel = nil
	s.active--
	s.hosts[t.host]--
	if s.hosts[t.host] == 0 {
		delete(s.hosts, t.host)
	}
	if r.interrupted || s.targets[t.id] != t {
		return
	}
	key := bucket{t.id, t.version, r.minute}
	value := s.minutes[key]
	if r.ok {
		value.success++
	} else {
		value.failure++
	}
	s.minutes[key] = value
}

const expirySQL = "DELETE FROM monitor_minutes WHERE minute < date_trunc('minute',now()) - interval '720 hours'"

const saveSQL = `INSERT INTO monitor_minutes(monitor_id,minute,successes,failures,last_batch)
 SELECT id,$3,$4,$5,$6 FROM monitors WHERE id=$1 AND url_version=$2 FOR SHARE
 ON CONFLICT(monitor_id,minute) DO UPDATE SET
 successes=monitor_minutes.successes+excluded.successes, failures=monitor_minutes.failures+excluded.failures,
 last_batch=excluded.last_batch WHERE monitor_minutes.last_batch<>excluded.last_batch`

func (s *scheduler) flush(ctx context.Context, before time.Time) error {

	keys := []bucket{}
	id := uuid.NewString()
	for k := range s.minutes {
		if k.minute.Before(before) {
			keys = append(keys, k)

		}
	}
	if len(keys) == 0 {
		return nil
	}
	// A pgx batch runs in one implicit transaction. Replay the same ID on a retry.
	for attempt := 0; ; attempt++ {
		batch := &pgx.Batch{}
		for _, k := range keys {
			v := s.minutes[k]
			batch.Queue(saveSQL, k.id, k.version, k.minute, v.success, v.failure, id)
		}
		err := s.conn.SendBatch(ctx, batch).Close()
		if err == nil {
			break
		}
		if attempt >= 2 || s.conn.IsClosed() || ctx.Err() != nil {
			return err
		}
	}
	for _, k := range keys {
		delete(s.minutes, k)
	}
	return nil
}

func (s *scheduler) run(ctx context.Context, dsn string) error {
	changed := make(chan struct{}, 1)
	go listen(ctx, dsn, changed)
	if err := s.reconcile(ctx); err != nil {
		return err
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	var lastMinute, lastCleanup time.Time
	for {
		select {
		case <-ctx.Done():
			// Completed observations are retained. Checks interrupted by shutdown are skipped.
			for s.active > 0 {
				r := <-s.results
				s.record(r)
			}
			drain, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return s.flush(drain, time.Now().Add(time.Minute))
		case <-changed:
			if err := s.reconcile(ctx); err != nil {
				return err
			}
		case r := <-s.results:
			s.record(r)
		case now := <-tick.C:
			s.dispatch(ctx, now)
			minute := now.UTC().Truncate(time.Minute)
			if minute != lastMinute {
				if err := s.flush(ctx, minute); err != nil {
					return err
				}
				log.Printf("skipped=%d max_start_delay=%s active=%d", s.skipped, s.lag, s.active)
				s.skipped = 0
				s.lag = 0
				lastMinute = minute
			}
			if now.Sub(lastCleanup) >= time.Hour {
				if _, err := s.conn.Exec(ctx, expirySQL); err != nil {
					return err
				}
				lastCleanup = now
			}
		}
	}
}

func listen(ctx context.Context, dsn string, changed chan<- struct{}) {
	wake := func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	}
	for ctx.Err() == nil {
		c, err := pgx.Connect(ctx, dsn)
		if err == nil {
			_, err = c.Exec(ctx, "LISTEN monitor_changes")
			if err == nil {
				wake()
			}
			for err == nil {
				_, err = c.WaitForNotification(ctx)
				if err == nil {
					wake()
				}
			}
			c.Close(context.Background())
		}
		if ctx.Err() != nil {
			return
		}
		log.Printf("monitor listener reconnect: %v", err)
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func run(ctx context.Context) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL required")
	}
	db, err := storage.Open(dsn)
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		return err
	}
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer c.Close(context.Background())
	var leader bool
	if err := c.QueryRow(ctx, "SELECT pg_try_advisory_lock(753196285)").Scan(&leader); err != nil {
		return err
	}
	if !leader {
		return errors.New("another worker is active")
	}
	s := scheduler{conn: c, targets: map[string]*target{}, hosts: map[string]int{}, minutes: map[bucket]counts{}, results: make(chan result, 64)}
	return s.run(ctx, dsn)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}
