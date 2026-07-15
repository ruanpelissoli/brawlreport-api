// Package bsclient provides a Brawl Stars API client.
// This file defines typed errors so callers can use errors.Is/As for control
// flow without hard-coding HTTP status codes.
package bsclient

import (
	"errors"
	"fmt"
)

// Sentinel errors for use with errors.Is().
var (
	ErrNotFound           = errors.New("bsclient: resource not found (404)")
	ErrAccessDenied       = errors.New("bsclient: access denied (403) — invalid token or IP not whitelisted")
	ErrBadRequest         = errors.New("bsclient: bad request (400) — check tag format / parameters")
	ErrRateLimited        = errors.New("bsclient: rate limited (429)")
	ErrServerError        = errors.New("bsclient: server error (500)")
	ErrServiceUnavailable = errors.New("bsclient: service unavailable (503) — possibly in-game maintenance")
)

// APIError is a rich error type that carries the HTTP status code, the reason
// field from the Supercell error body, and the full message. It wraps one of
// the sentinel errors above so callers can use errors.Is().
//
// 403 disambiguation: Supercell returns the same "accessDenied" reason for
// both an invalid API token and a server IP that is not whitelisted on the
// key. The raw Message from the API response often hints at the cause (e.g.
// "IP address not whitelisted" vs "Invalid authorization"). Always inspect
// Message when troubleshooting 403s.
type APIError struct {
	StatusCode int
	Reason     string
	Message    string
	sentinel   error
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("bsclient: HTTP %d — reason=%q message=%q", e.StatusCode, e.Reason, e.Message)
}

// Unwrap returns the sentinel error so errors.Is works correctly.
func (e *APIError) Unwrap() error {
	return e.sentinel
}

// apiErrorBody is the JSON shape Supercell returns for error responses.
type apiErrorBody struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// newAPIError constructs an *APIError from a status code and parsed body,
// choosing the correct sentinel based on the status code.
func newAPIError(statusCode int, body apiErrorBody) *APIError {
	var sentinel error
	switch statusCode {
	case 400:
		sentinel = ErrBadRequest
	case 403:
		sentinel = ErrAccessDenied
	case 404:
		sentinel = ErrNotFound
	case 429:
		sentinel = ErrRateLimited
	case 503:
		sentinel = ErrServiceUnavailable
	default:
		sentinel = ErrServerError
	}
	return &APIError{
		StatusCode: statusCode,
		Reason:     body.Reason,
		Message:    body.Message,
		sentinel:   sentinel,
	}
}
