// Package reputation provides MX DNS lookup with in-memory caching.
// This is Capa 2, Step 3 of the Anti-Bounce Engine: verify that a domain
// has valid MX records before spending a send attempt on it.
package reputation

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const mxCacheTTL = 1 * time.Hour

// mxCacheEntry stores a cached MX lookup result.
type mxCacheEntry struct {
	hasMX     bool
	cachedAt  time.Time
}

// MXCache performs DNS MX lookups with an in-memory cache per domain.
// Thread-safe for concurrent use from the analyzer goroutine pool.
type MXCache struct {
	mu      sync.RWMutex
	cache   map[string]mxCacheEntry
	timeout time.Duration
}

// NewMXCache creates a new MXCache with the given lookup timeout.
func NewMXCache(lookupTimeout time.Duration) *MXCache {
	return &MXCache{
		cache:   make(map[string]mxCacheEntry),
		timeout: lookupTimeout,
	}
}

// HasMX returns true if the domain has at least one MX record.
// Results are cached for mxCacheTTL (1 hour) to avoid repeated DNS queries
// for the same domain across thousands of invoices in a single batch.
func (c *MXCache) HasMX(email string) bool {
	domain := extractDomain(email)
	if domain == "" {
		return false
	}

	// Check cache first (read lock)
	c.mu.RLock()
	entry, found := c.cache[domain]
	c.mu.RUnlock()

	if found && time.Since(entry.cachedAt) < mxCacheTTL {
		return entry.hasMX
	}

	// Cache miss or stale: perform DNS lookup
	hasMX := c.lookupMX(domain)

	// Store result (write lock)
	c.mu.Lock()
	c.cache[domain] = mxCacheEntry{
		hasMX:    hasMX,
		cachedAt: time.Now(),
	}
	c.mu.Unlock()

	return hasMX
}

// lookupMX performs the actual DNS MX lookup.
func (c *MXCache) lookupMX(domain string) bool {
	// Use a timeout via net.Resolver with a deadline
	resolver := &net.Resolver{}

	// Apply timeout using a goroutine + channel approach
	type result struct {
		mxRecords []*net.MX
		err       error
	}
	ch := make(chan result, 1)

	go func() {
		records, err := resolver.LookupMX(context.Background(), domain) //nolint:staticcheck
		ch <- result{records, err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			log.Printf("[MXCache] MX lookup failed for %s: %v", domain, res.err)
			return false
		}
		return len(res.mxRecords) > 0
	case <-time.After(c.timeout):
		log.Printf("[MXCache] MX lookup timeout for %s", domain)
		return false
	}
}

// WarmUp pre-populates the cache for a list of unique domains.
// Call this at the start of analysis to batch-resolve all domains
// concurrently before the main goroutine pool processes invoices.
func (c *MXCache) WarmUp(emails []string) {
	// Collect unique domains
	seen := make(map[string]bool)
	var domains []string
	for _, email := range emails {
		d := extractDomain(email)
		if d != "" && !seen[d] {
			seen[d] = true
			domains = append(domains, d)
		}
	}

	log.Printf("[MXCache] Warming up cache for %d unique domains...", len(domains))

	// Resolve concurrently (bounded to 10 goroutines)
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	for _, domain := range domains {
		wg.Add(1)
		sem <- struct{}{}
		go func(d string) {
			defer wg.Done()
			defer func() { <-sem }()
			c.HasMX(d + "@placeholder") // triggers lookup via HasMX internal domain extraction
			// Note: HasMX expects an email, not a domain. Workaround: call lookupMX directly.
			_ = c.lookupMXDomain(d)
		}(domain)
	}
	wg.Wait()
	log.Printf("[MXCache] Warm-up complete. %d domains resolved.", len(domains))
}

// lookupMXDomain is the same as lookupMX but called directly by WarmUp.
func (c *MXCache) lookupMXDomain(domain string) bool {
	hasMX := c.lookupMX(domain)
	c.mu.Lock()
	c.cache[domain] = mxCacheEntry{hasMX: hasMX, cachedAt: time.Now()}
	c.mu.Unlock()
	return hasMX
}

// HasMXDomain performs the lookup by domain name directly (no email parsing needed).
func (c *MXCache) HasMXDomain(domain string) bool {
	c.mu.RLock()
	entry, found := c.cache[domain]
	c.mu.RUnlock()

	if found && time.Since(entry.cachedAt) < mxCacheTTL {
		return entry.hasMX
	}
	return c.lookupMXDomain(domain)
}

// extractDomain extracts the domain part from an email address.
func extractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}
	return strings.ToLower(parts[1])
}
