package ratelimit

import "testing"

func TestLimiter_AllowsWithinBurst(t *testing.T) {
	l := NewLimiter(10, 5, 10, 5)
	defer l.Stop()

	for i := 0; i < 5; i++ {
		if !l.Allow("1.2.3.4", "key1") {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}
}

func TestLimiter_RejectsOverBurst(t *testing.T) {
	l := NewLimiter(1, 2, 1, 2)
	defer l.Stop()

	// Consume burst
	l.Allow("1.2.3.4", "key1")
	l.Allow("1.2.3.4", "key1")

	// Should be rejected now
	if l.Allow("1.2.3.4", "key1") {
		t.Error("expected rejection after burst exhausted")
	}
}

func TestLimiter_DifferentIPsIndependent(t *testing.T) {
	l := NewLimiter(1, 1, 100, 100)
	defer l.Stop()

	if !l.Allow("1.1.1.1", "") {
		t.Error("first IP should be allowed")
	}
	if !l.Allow("2.2.2.2", "") {
		t.Error("second IP should be allowed independently")
	}
}

func TestLimiter_PerKeyLimiting(t *testing.T) {
	l := NewLimiter(100, 100, 1, 1)
	defer l.Stop()

	if !l.Allow("1.1.1.1", "key1") {
		t.Error("first request for key1 should be allowed")
	}
	if l.Allow("2.2.2.2", "key1") {
		t.Error("second request for key1 (different IP) should be rejected by per-key limit")
	}
}

func TestLimiter_NoKeySkipsKeyCheck(t *testing.T) {
	l := NewLimiter(100, 100, 1, 1)
	defer l.Stop()

	for i := 0; i < 5; i++ {
		if !l.Allow("1.1.1.1", "") {
			t.Fatalf("request %d with empty key should skip per-key check", i+1)
		}
	}
}

func TestLimiter_Status(t *testing.T) {
	l := NewLimiter(10, 20, 5, 10)
	defer l.Stop()

	l.Allow("1.1.1.1", "key1")
	status := l.Status()

	if status["ip_rps"] != 10.0 {
		t.Errorf("expected ip_rps=10, got %v", status["ip_rps"])
	}
	if status["per_key_burst"] != 10 {
		t.Errorf("expected per_key_burst=10, got %v", status["per_key_burst"])
	}
	if status["active_ip_limiters"].(int) < 1 {
		t.Error("expected at least 1 active IP limiter")
	}
}
