package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestProbe(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
		}
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(200)
		case "/empty":
			w.WriteHeader(204)
		case "/redirect":
			http.Redirect(w, r, "/ok", 301)
		case "/slow":
			<-r.Context().Done()
		default:
			w.WriteHeader(500)
		}
	}))
	defer s.Close()
	if probe(context.Background(), s.URL+"/ok") {
		t.Fatal("production dialer reached loopback")
	}
	original := client
	client = &http.Client{Transport: s.Client().Transport, CheckRedirect: original.CheckRedirect, Timeout: 300 * time.Millisecond}
	defer func() { client = original }()
	for _, p := range []string{"/ok", "/empty", "/redirect", "/error", "/slow"} {
		if got := probe(context.Background(), s.URL+p); got != (p == "/ok") {
			t.Errorf("%s: %t", p, got)
		}
	}
}
func TestPublicIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "169.254.169.254", "10.0.0.1", "100.64.0.1", "::ffff:192.168.1.1", "fe80::1", "fd00::1", "64:ff9b::a00:1", "2002::1", "ff02::1"} {
		if publicIP(netip.MustParseAddr(s)) {
			t.Errorf("allowed %s", s)
		}
	}
}
func TestSkipBusyTicks(t *testing.T) {
	now := time.Now()
	targetA := &target{id: "a", host: "example.com", interval: time.Second, next: now.Add(-time.Second), cancel: func() {}}
	s := scheduler{targets: map[string]*target{"a": targetA}}
	s.dispatch(context.Background(), now)
	if s.skipped != 2 || len(s.minutes) != 0 || !targetA.next.Equal(now.Add(time.Second)) {
		t.Fatal("busy ticks counted as checks or scheduled for catchup")
	}
}
