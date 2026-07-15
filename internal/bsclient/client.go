package bsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/brawlreport/api/internal/config"
)

const (
	defaultTimeout    = 10 * time.Second
	maxRetries        = 5
	backoffBase       = 1 * time.Second
	backoffMax        = 60 * time.Second
	cooldownBase      = 5 * time.Second  // initial cooldown when a token gets 429
	cooldownMax       = 60 * time.Second // maximum per-token cooldown
)

// Client is a typed, rate-limited HTTP client for the Brawl Stars API.
// It manages a fleet of API tokens, applies per-token token-bucket rate
// limiting, and retries 429/503 responses with exponential backoff.
type Client struct {
	httpClient *http.Client
	pool       *TokenPool
	baseURL    string
	logger     *slog.Logger
}

// NewClient constructs a Client from application config and a logger.
func NewClient(cfg *config.Config, logger *slog.Logger) *Client {
	pool := NewTokenPool(cfg.BSAPITokens, cfg.BSAPIRatePerToken)
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		pool:       pool,
		baseURL:    strings.TrimRight(cfg.BSAPIBaseURL, "/"),
		logger:     logger,
	}
}

// EncodeTag uppercases a player/club tag and URL-encodes the '#' as '%23'.
// The raw tag "#2PP0LG" becomes "%232PP0LG" for use in URL paths.
func EncodeTag(tag string) string {
	tag = strings.ToUpper(strings.TrimSpace(tag))
	return strings.ReplaceAll(tag, "#", "%23")
}

// ─── Public API methods ──────────────────────────────────────────────────────

// GetPlayer fetches a player profile by tag.
func (c *Client) GetPlayer(ctx context.Context, tag string) (*Player, error) {
	var out Player
	if err := c.do(ctx, http.MethodGet, "/players/"+EncodeTag(tag), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBattleLog fetches the battle log for a player by tag.
func (c *Client) GetBattleLog(ctx context.Context, tag string) (*BattleLogResponse, error) {
	var out BattleLogResponse
	if err := c.do(ctx, http.MethodGet, "/players/"+EncodeTag(tag)+"/battlelog", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBrawlers returns the full list of brawlers.
func (c *Client) GetBrawlers(ctx context.Context) (*BrawlerList, error) {
	var out BrawlerList
	if err := c.do(ctx, http.MethodGet, "/brawlers", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBrawler returns a single brawler by numeric ID.
func (c *Client) GetBrawler(ctx context.Context, id int) (*Brawler, error) {
	var out Brawler
	path := fmt.Sprintf("/brawlers/%d", id)
	if err := c.do(ctx, http.MethodGet, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEventRotation returns the current event rotation.
func (c *Client) GetEventRotation(ctx context.Context) (EventRotation, error) {
	var out EventRotation
	if err := c.do(ctx, http.MethodGet, "/events/rotation", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetRankingsPlayers returns the top player rankings for a country code.
// Use "global" for the global leaderboard.
func (c *Client) GetRankingsPlayers(ctx context.Context, countryCode string) (*RankingList, error) {
	var out RankingList
	path := fmt.Sprintf("/rankings/%s/players", countryCode)
	if err := c.do(ctx, http.MethodGet, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRankingsBrawler returns the top-player rankings for a specific brawler.
func (c *Client) GetRankingsBrawler(ctx context.Context, countryCode string, brawlerID int) (*RankingList, error) {
	var out RankingList
	path := fmt.Sprintf("/rankings/%s/brawlers/%d", countryCode, brawlerID)
	if err := c.do(ctx, http.MethodGet, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetClub fetches a club by tag.
func (c *Client) GetClub(ctx context.Context, tag string) (*Club, error) {
	var out Club
	if err := c.do(ctx, http.MethodGet, "/clubs/"+EncodeTag(tag), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetClubMembers fetches the paginated member list for a club.
func (c *Client) GetClubMembers(ctx context.Context, tag string) (*ClubMemberList, error) {
	var out ClubMemberList
	if err := c.do(ctx, http.MethodGet, "/clubs/"+EncodeTag(tag)+"/members", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Internal request machinery ─────────────────────────────────────────────

// do performs a single API call with automatic retry on 429/503.
// It acquires an API token from the pool, sets the Authorization header, and
// decodes the response into target. On 429/503 it marks the token, sleeps for
// an exponential backoff window, and retries up to maxRetries times.
func (c *Client) do(ctx context.Context, method, path string, target any) error {
	url := c.baseURL + path

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			sleep := backoffDuration(attempt)
			c.logger.Debug("retrying after backoff",
				"attempt", attempt,
				"sleep", sleep.String(),
				"url", url,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sleep):
			}
		}

		token, err := c.pool.Acquire(ctx)
		if err != nil {
			return fmt.Errorf("bsclient: acquire token: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return fmt.Errorf("bsclient: build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("bsclient: execute request: %w", err)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()
			if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
				return fmt.Errorf("bsclient: decode response: %w", err)
			}
			return nil
		}

		// Non-2xx — decode error body.
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var errBody apiErrorBody
		_ = json.Unmarshal(body, &errBody)
		apiErr := newAPIError(resp.StatusCode, errBody)

		c.logger.Warn("API error",
			"status", resp.StatusCode,
			"reason", errBody.Reason,
			"message", errBody.Message,
			"url", url,
			"attempt", attempt,
		)

		if resp.StatusCode == 429 || resp.StatusCode == 503 {
			cooldown := cooldownDuration(attempt)
			c.pool.MarkRateLimited(token, cooldown)
			lastErr = apiErr
			continue // retry
		}

		// Non-retryable error.
		return apiErr
	}

	return fmt.Errorf("bsclient: max retries exceeded for %s: %w", url, lastErr)
}

// backoffDuration returns how long to sleep before retry attempt n.
// Base: 1s, multiplied by 2^(n-1), capped at backoffMax.
func backoffDuration(attempt int) time.Duration {
	d := backoffBase
	for i := 1; i < attempt; i++ {
		d *= 2
		if d > backoffMax {
			return backoffMax
		}
	}
	return d
}

// cooldownDuration returns how long a token should be marked unavailable
// after a 429/503. Grows with subsequent attempts to back off further.
func cooldownDuration(attempt int) time.Duration {
	d := cooldownBase
	for i := 0; i < attempt; i++ {
		d *= 2
		if d > cooldownMax {
			return cooldownMax
		}
	}
	return d
}
