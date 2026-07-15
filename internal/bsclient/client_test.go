package bsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/brawlreport/api/internal/config"
	"github.com/brawlreport/api/internal/logger"
)

// ─── EncodeTag ───────────────────────────────────────────────────────────────

func TestEncodeTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"#2PP0LG", "%232PP0LG"},
		{"2PP0LG", "2PP0LG"},           // no leading #
		{"#2pp0lg", "%232PP0LG"},       // lowercase input is uppercased
		{"#ABC123", "%23ABC123"},
		{" #ABC ", "%23ABC"},           // whitespace trimmed
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := EncodeTag(tt.input)
			if got != tt.want {
				t.Errorf("EncodeTag(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func makeClient(t *testing.T, baseURL string, tokens []string) *Client {
	t.Helper()
	cfg := &config.Config{
		BSAPIBaseURL:      baseURL,
		BSAPITokens:       tokens,
		BSAPIRatePerToken: 100, // high rate so tests aren't throttled
		LogLevel:          "debug",
	}
	log := logger.New(cfg)
	return NewClient(cfg, log)
}

// ─── 429 retry ───────────────────────────────────────────────────────────────

func TestClientRetryOn429(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			// Return 429 for the first two calls.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(apiErrorBody{
				Reason:  "rateLimited",
				Message: "too many requests",
			})
			return
		}
		// Third call succeeds.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Player{Tag: "#2PP0LG", Name: "TestPlayer"})
	}))
	defer srv.Close()

	client := makeClient(t, srv.URL, []string{"tok1"})
	p, err := client.GetPlayer(context.Background(), "#2PP0LG")
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if p.Tag != "#2PP0LG" {
		t.Errorf("unexpected player tag: %s", p.Tag)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls (2 x 429 + 1 success), got %d", calls.Load())
	}
}

// ─── 503 retry ───────────────────────────────────────────────────────────────

func TestClient503Retry(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(apiErrorBody{Reason: "serviceUnavailable"})
			return
		}
		_ = json.NewEncoder(w).Encode(Player{Tag: "#CLUB1", Name: "Player"})
	}))
	defer srv.Close()

	client := makeClient(t, srv.URL, []string{"tok1"})
	_, err := client.GetPlayer(context.Background(), "#CLUB1")
	if err != nil {
		t.Fatalf("expected success after 503+retry, got: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", calls.Load())
	}
}

// ─── 404 → ErrNotFound ──────────────────────────────────────────────────────

func TestClientNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(apiErrorBody{
			Reason:  "notFound",
			Message: "Player not found",
		})
	}))
	defer srv.Close()

	client := makeClient(t, srv.URL, []string{"tok1"})
	_, err := client.GetPlayer(context.Background(), "#NOTEXIST")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// ─── Token rotation: first token always 429, second succeeds ─────────────────

func TestClientTokenRotation(t *testing.T) {
	// Map each token to its call counter.
	callsToken1 := &atomic.Int32{}
	callsToken2 := &atomic.Int32{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := ""
		if len(auth) > 7 {
			token = auth[7:] // strip "Bearer "
		}

		switch token {
		case "token1":
			callsToken1.Add(1)
			// token1 always rate-limits.
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(apiErrorBody{Reason: "rateLimited"})
		case "token2":
			callsToken2.Add(1)
			// token2 always succeeds.
			_ = json.NewEncoder(w).Encode(Player{Tag: "#ROT", Name: "Rotation"})
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()

	client := makeClient(t, srv.URL, []string{"token1", "token2"})
	p, err := client.GetPlayer(context.Background(), "#ROT")
	if err != nil {
		t.Fatalf("expected success via token2, got: %v", err)
	}
	if p.Tag != "#ROT" {
		t.Errorf("unexpected player tag: %s", p.Tag)
	}
	if callsToken1.Load() == 0 {
		t.Error("expected token1 to be tried at least once")
	}
	if callsToken2.Load() == 0 {
		t.Error("expected token2 to be tried at least once")
	}
}

// ─── 403 → ErrAccessDenied ──────────────────────────────────────────────────

func TestClientAccessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(apiErrorBody{
			Reason:  "accessDenied",
			Message: "Invalid authorization",
		})
	}))
	defer srv.Close()

	client := makeClient(t, srv.URL, []string{"tok1"})
	_, err := client.GetPlayer(context.Background(), "#P1")
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got: %v", err)
	}
}

// ─── BattleLog shape: friendly has nil TrophyChange ─────────────────────────

func TestBattleLogFriendlyNilTrophyChange(t *testing.T) {
	// Verify that our model correctly represents a friendly battle (no trophyChange)
	// via a nil pointer — not a zero int.
	rawJSON := `{
		"items": [{
			"battleTime": "20230115T143000.000Z",
			"event": { "id": 1, "mode": "gemGrab", "map": "Hard Rock Mine" },
			"battle": {
				"mode": "gemGrab",
				"type": "friendly",
				"duration": 90
			}
		}],
		"paging": { "cursors": {} }
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, rawJSON)
	}))
	defer srv.Close()

	client := makeClient(t, srv.URL, []string{"tok1"})
	log, err := client.GetBattleLog(context.Background(), "#P1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(log.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(log.Items))
	}
	battle := log.Items[0].Battle
	if battle.Type != "friendly" {
		t.Errorf("expected type=friendly, got %s", battle.Type)
	}
	if battle.TrophyChange != nil {
		t.Errorf("expected nil TrophyChange for friendly battle, got %v", *battle.TrophyChange)
	}
	if battle.Result != nil {
		t.Errorf("expected nil Result for friendly battle, got %v", *battle.Result)
	}
}
