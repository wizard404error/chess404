package rate_limit

import (
	"testing"
	"time"
)

func TestFloodDetectorBasic(t *testing.T) {
	fd := NewFloodDetector(5, 1*time.Second, 10*time.Second)

	for i := 0; i < 4; i++ {
		if fd.IsFlood("1.2.3.4") {
			t.Fatalf("should not be flood at request %d", i+1)
		}
	}

	if !fd.IsFlood("1.2.3.4") {
		t.Error("should be flood at request 5")
	}
}

func TestFloodDetectorBan(t *testing.T) {
	fd := NewFloodDetector(3, 1*time.Second, 5*time.Second)

	fd.IsFlood("1.2.3.4")
	fd.IsFlood("1.2.3.4")
	fd.IsFlood("1.2.3.4")

	if !fd.IsBanned("1.2.3.4") {
		t.Error("should be banned after exceeding threshold")
	}

	fd.Unban("1.2.3.4")
	if fd.IsBanned("1.2.3.4") {
		t.Error("should not be banned after unban")
	}
}

func TestFloodDetectorManualBan(t *testing.T) {
	fd := NewFloodDetector(100, 1*time.Second, 1*time.Second)

	fd.Ban("1.2.3.4", 10*time.Second)

	if !fd.IsBanned("1.2.3.4") {
		t.Error("should be banned after manual ban")
	}
}

func TestFloodDetectorDifferentIPs(t *testing.T) {
	fd := NewFloodDetector(3, 1*time.Second, 1*time.Second)

	fd.IsFlood("1.2.3.4")
	fd.IsFlood("1.2.3.4")
	fd.IsFlood("1.2.3.4")

	if fd.IsFlood("5.6.7.8") {
		t.Error("different IP should not be affected")
	}
}

func TestFloodDetectorStats(t *testing.T) {
	fd := NewFloodDetector(10, 1*time.Second, 1*time.Second)

	fd.IsFlood("1.2.3.4")
	fd.IsFlood("5.6.7.8")

	stats := fd.Stats()
	if stats.TrackedIPs != 2 {
		t.Errorf("expected 2 tracked IPs, got %d", stats.TrackedIPs)
	}
}

func TestCompositeRateLimiter(t *testing.T) {
	crl := NewCompositeRateLimiter(5, 1*time.Second, 20, 1*time.Minute)

	for i := 0; i < 5; i++ {
		if !crl.Allow("test-key") {
			t.Fatalf("should allow request %d", i+1)
		}
	}

	if crl.Allow("test-key") {
		t.Error("should deny after fast limit")
	}
}

func TestCompositeRateLimiterDifferentKeys(t *testing.T) {
	crl := NewCompositeRateLimiter(2, 1*time.Second, 10, 1*time.Minute)

	crl.Allow("key-1")
	crl.Allow("key-1")

	if !crl.Allow("key-2") {
		t.Error("different key should be allowed")
	}
}

func TestCompositeRateLimiterStats(t *testing.T) {
	crl := NewCompositeRateLimiter(5, 1*time.Second, 20, 1*time.Minute)

	crl.Allow("key-1")
	crl.Allow("key-2")

	stats := crl.Stats()
	if stats.FastTrackedIPs != 2 {
		t.Errorf("expected 2 fast tracked IPs, got %d", stats.FastTrackedIPs)
	}
}
