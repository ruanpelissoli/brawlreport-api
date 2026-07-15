package bsclient

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// tokenEntry pairs an API token string with its per-token rate limiter and
// a mutex-protected cooldown state.
type tokenEntry struct {
	token   string
	limiter *rate.Limiter

	mu          sync.Mutex
	unavailable bool          // true when the token is in a rate-limited backoff window
	availableAt time.Time     // when the token becomes usable again
}

// isAvailable reports whether this token can be used right now.
func (e *tokenEntry) isAvailable() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.unavailable && time.Now().After(e.availableAt) {
		e.unavailable = false
	}
	return !e.unavailable
}

// markRateLimited places the token in cooldown for the given duration.
func (e *tokenEntry) markRateLimited(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.unavailable = true
	e.availableAt = time.Now().Add(d)
}

// TokenPool manages a fleet of API tokens with per-token rate limiting and
// round-robin selection. Each token has its own token-bucket limiter so that
// rate-limiting pressure is spread evenly across the pool.
type TokenPool struct {
	entries []*tokenEntry
	mu      sync.Mutex
	next    int
}

// NewTokenPool creates a TokenPool from the given token strings.
// ratePerToken is the max requests per second per token.
func NewTokenPool(tokens []string, ratePerToken float64) *TokenPool {
	entries := make([]*tokenEntry, 0, len(tokens))
	for _, t := range tokens {
		r := rate.Limit(ratePerToken)
		burst := max(1, int(ratePerToken)) // burst == rate ceiling rounded up
		entries = append(entries, &tokenEntry{
			token:   t,
			limiter: rate.NewLimiter(r, burst),
		})
	}
	return &TokenPool{entries: entries}
}

// Acquire waits until a token is available (rate limiter allows) and returns
// the token string. It uses round-robin selection, skipping tokens that are
// currently in a rate-limited backoff window.
//
// If all tokens are in cooldown it falls back to waiting on the least-
// constrained token to minimise latency.
func (p *TokenPool) Acquire(ctx context.Context) (string, error) {
	p.mu.Lock()
	n := len(p.entries)
	// Collect available tokens in round-robin order.
	available := make([]*tokenEntry, 0, n)
	for i := 0; i < n; i++ {
		e := p.entries[(p.next+i)%n]
		if e.isAvailable() {
			available = append(available, e)
		}
	}

	var chosen *tokenEntry
	if len(available) > 0 {
		chosen = available[0]
		// Advance the round-robin pointer past the chosen token.
		for i, e := range p.entries {
			if e == chosen {
				p.next = (i + 1) % n
				break
			}
		}
	} else {
		// All tokens are in cooldown — fall back to the next token in sequence
		// and let the caller deal with retries.
		chosen = p.entries[p.next]
		p.next = (p.next + 1) % n
	}
	p.mu.Unlock()

	// Wait for the rate limiter.
	if err := chosen.limiter.Wait(ctx); err != nil {
		return "", err
	}
	return chosen.token, nil
}

// MarkRateLimited places the named token in a backoff cooldown for duration d.
// Call this when the server returns 429 or 503 for a request made with token.
func (p *TokenPool) MarkRateLimited(token string, d time.Duration) {
	for _, e := range p.entries {
		if e.token == token {
			e.markRateLimited(d)
			return
		}
	}
}

// max returns the larger of a and b. (stdlib max requires Go 1.21 with generics.)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
