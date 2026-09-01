package main

import (
	"testing"
	"time"
)

func TestHeartbeatPolicyKeepsServiceAliveDuringNormalUserPause(t *testing.T) {
	started := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	lastBeat := started.Add(1 * time.Minute)
	now := lastBeat.Add(5 * time.Minute)

	if shouldCloseForHeartbeat(now, started, true, lastBeat.UnixNano()) {
		t.Fatal("desktop service closed after a short browser heartbeat pause")
	}
}

func TestHeartbeatPolicyEventuallyClosesAbandonedService(t *testing.T) {
	started := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	lastBeat := started.Add(1 * time.Minute)
	now := lastBeat.Add(browserHeartbeatGrace + time.Second)

	if !shouldCloseForHeartbeat(now, started, true, lastBeat.UnixNano()) {
		t.Fatal("desktop service did not close after the abandoned-browser grace period")
	}
}

func TestHeartbeatPolicyAllowsSlowInitialBrowserStartup(t *testing.T) {
	started := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	now := started.Add(3 * time.Minute)

	if shouldCloseForHeartbeat(now, started, false, 0) {
		t.Fatal("desktop service closed before a slow initial browser startup could connect")
	}
}
