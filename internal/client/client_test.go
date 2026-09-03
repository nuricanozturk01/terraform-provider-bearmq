package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := New(Config{Endpoint: srv.URL, APIKey: "test-key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNew_validation(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"missing endpoint", Config{APIKey: "k"}, false},
		{"bad scheme", Config{Endpoint: "ftp://x", APIKey: "k"}, false},
		{"no creds", Config{Endpoint: "http://x"}, false},
		{"both creds", Config{Endpoint: "http://x", APIKey: "k", Token: "t"}, false},
		{"api key ok", Config{Endpoint: "http://x", APIKey: "k"}, true},
		{"token ok", Config{Endpoint: "http://x", Token: "t"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.cfg)
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestClient_sendsAuthAndParsesBody(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-KEY"); got != "test-key" {
			t.Errorf("X-API-KEY = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Queue{ID: "q1", Name: "orders"})
	}))

	q, err := c.CreateQueue(context.Background(), "v1", QueueRequest{Name: "orders"})
	if err != nil {
		t.Fatalf("CreateQueue: %v", err)
	}
	if q.ID != "q1" || q.Name != "orders" {
		t.Fatalf("got %+v", q)
	}
}

func TestClient_errorStatusMapping(t *testing.T) {
	cases := []struct {
		status   int
		sentinel error
	}{
		{http.StatusNotFound, ErrNotFound},
		{http.StatusConflict, ErrConflict},
		{http.StatusPaymentRequired, ErrQuota},
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrUnauthorized},
	}
	for _, tc := range cases {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_ = json.NewEncoder(w).Encode(apiErrorResponse{
				Status: tc.status, Error: http.StatusText(tc.status), Message: "boom", Path: "/x",
			})
		}))

		_, err := c.GetQueue(context.Background(), "v1", "q1")
		if !errors.Is(err, tc.sentinel) {
			t.Fatalf("status %d: got %v, want errors.Is %v", tc.status, err, tc.sentinel)
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Message != "boom" {
			t.Fatalf("status %d: APIError not surfaced: %v", tc.status, err)
		}
	}
}

func TestClient_retriesOn503ThenSucceeds(t *testing.T) {
	var calls int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Exchange{ID: "e1", Name: "orders", Type: "TOPIC"})
	}))

	e, err := c.GetExchange(context.Background(), "v1", "e1")
	if err != nil {
		t.Fatalf("GetExchange: %v", err)
	}
	if e.ID != "e1" || atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("calls=%d exchange=%+v", calls, e)
	}
}

func TestClient_doesNotRetry404(t *testing.T) {
	var calls int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))

	_, err := c.GetVHost(context.Background(), "v1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("want 1 call, got %d", calls)
	}
}

func TestClient_paginationFindsOnSecondPage(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		body := pageResponse[VHost]{TotalPages: 2, Size: 100}
		if page == "0" {
			body.Content = []VHost{{ID: "a", Name: "alpha"}}
			body.Number = 0
		} else {
			body.Content = []VHost{{ID: "b", Name: "beta"}}
			body.Number = 1
		}
		_ = json.NewEncoder(w).Encode(body)
	}))

	vh, err := c.FindVHostByName(context.Background(), "beta")
	if err != nil {
		t.Fatalf("FindVHostByName: %v", err)
	}
	if vh.ID != "b" {
		t.Fatalf("got %+v", vh)
	}
}

func TestClient_paginationNotFound(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pageResponse[VHost]{
			Content: []VHost{{ID: "a", Name: "alpha"}}, TotalPages: 1,
		})
	}))

	_, err := c.FindVHostByName(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestClient_bearerTokenHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer jwt-abc" {
			t.Errorf("Authorization = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := New(Config{Endpoint: srv.URL, Token: "jwt-abc", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.DeleteQueue(context.Background(), "v1", "q1"); err != nil {
		t.Fatalf("DeleteQueue: %v", err)
	}
}
