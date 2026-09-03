package client

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by the client. Callers use errors.Is to branch on
// them — in particular ErrNotFound drives Terraform state removal on Read.
var (
	ErrNotFound     = errors.New("bearmq: resource not found")
	ErrConflict     = errors.New("bearmq: resource already exists")
	ErrQuota        = errors.New("bearmq: plan quota exceeded")
	ErrUnauthorized = errors.New("bearmq: unauthorized")
)

// APIError carries the parsed body of a non-2xx BearMQ response. It wraps one of
// the sentinel errors above (when the status maps to one) so both errors.Is
// checks and a human-readable message work.
type APIError struct {
	StatusCode int
	Status     string
	Message    string
	Path       string
	sentinel   error
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Status
	}
	if e.Path != "" {
		return fmt.Sprintf("bearmq API %d on %s: %s", e.StatusCode, e.Path, msg)
	}
	return fmt.Sprintf("bearmq API %d: %s", e.StatusCode, msg)
}

func (e *APIError) Unwrap() error { return e.sentinel }

// apiErrorResponse mirrors com.bearmq.api.common.dtos.ApiErrorResponse.
type apiErrorResponse struct {
	Status  int    `json:"status"`
	Error   string `json:"error"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

func newAPIError(statusCode int, body []byte) *APIError {
	parsed := decodeErrorBody(body)

	e := &APIError{
		StatusCode: statusCode,
		Status:     parsed.Error,
		Message:    parsed.Message,
		Path:       parsed.Path,
	}

	switch statusCode {
	case 404:
		e.sentinel = ErrNotFound
	case 409:
		e.sentinel = ErrConflict
	case 402:
		e.sentinel = ErrQuota
	case 401, 403:
		e.sentinel = ErrUnauthorized
	}
	return e
}
