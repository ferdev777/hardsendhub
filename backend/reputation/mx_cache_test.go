package reputation

import (
	"testing"
	"time"
)

func TestMXCache_HasMX(t *testing.T) {
	cache := NewMXCache(2 * time.Second)

	// Valid domain should return true
	if !cache.HasMX("user@gmail.com") {
		t.Error("Expected gmail.com to have MX records")
	}

	// Invalid domain should return false
	if cache.HasMX("user@this-is-a-fake-domain-for-testing-mx.local") {
		t.Error("Expected fake domain to NOT have MX records")
	}

	// Test caching behavior (second lookup should be very fast / instant)
	start := time.Now()
	if !cache.HasMX("user@gmail.com") {
		t.Error("Expected gmail.com to have MX records on cached lookup")
	}
	duration := time.Since(start)

	if duration > 10*time.Millisecond {
		t.Errorf("Cached lookup took too long: %v", duration)
	}
}

func TestMXCache_ExtractDomain(t *testing.T) {
	tests := []struct {
		email    string
		expected string
	}{
		{"test@gmail.com", "gmail.com"},
		{"test", ""},
		{"@domain.com", "domain.com"},
		{"TEST@GMAIL.COM", "gmail.com"},
	}

	for _, tt := range tests {
		result := extractDomain(tt.email)
		if result != tt.expected {
			t.Errorf("extractDomain(%q) = %q; want %q", tt.email, result, tt.expected)
		}
	}
}

func TestMXCache_WarmUp(t *testing.T) {
	cache := NewMXCache(2 * time.Second)

	emails := []string{
		"a@google.com",
		"b@google.com",
		"c@yahoo.com",
	}

	cache.WarmUp(emails)

	// Check if they are in cache
	cache.mu.RLock()
	_, googleOk := cache.cache["google.com"]
	_, yahooOk := cache.cache["yahoo.com"]
	cache.mu.RUnlock()

	if !googleOk {
		t.Error("Expected google.com to be in cache after warmup")
	}
	if !yahooOk {
		t.Error("Expected yahoo.com to be in cache after warmup")
	}
}
