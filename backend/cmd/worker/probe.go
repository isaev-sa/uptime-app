package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// Special-purpose ranges that IsGlobalUnicast includes but a public probe must not dial.
var reserved = func() []netip.Prefix {
	var out []netip.Prefix
	for _, s := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20", "2620:4f:8000::/48", "168.63.129.16/32"} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()
var globalV6 = netip.MustParsePrefix("2000::/3")

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || (ip.Is6() && !globalV6.Contains(ip)) {
		return false
	}
	for _, block := range reserved {
		if block.Contains(ip) {
			return false
		}
	}
	return true
}
func dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, errors.New("nonpublic destination")
		}
	}
	var dialer net.Dialer
	for _, ip := range ips {
		var conn net.Conn
		conn, err = dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	if err == nil {
		err = errors.New("no address")
	}
	return nil, err
}

var client = &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{DialContext: dialPublic, DisableCompression: true, IdleConnTimeout: 30 * time.Second, MaxResponseHeaderBytes: 65536}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func probe(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
