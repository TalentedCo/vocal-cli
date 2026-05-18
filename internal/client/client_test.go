package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateCallSendsAuthAndIdempotencyHeaders(t *testing.T) {
	var gotAuth, gotIdempotency string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotIdempotency = r.Header.Get("Idempotency-Key")
		if r.URL.Path != "/calls" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var payload CreateCallRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.MaxRetries != 0 {
			t.Fatalf("max retries = %d", payload.MaxRetries)
		}
		if payload.WebhookURL != "https://example.com/webhooks/vocal" {
			t.Fatalf("webhook url = %q", payload.WebhookURL)
		}
		if payload.WebhookHeaders["Authorization"] != "Bearer webhook-secret" {
			t.Fatalf("webhook headers = %#v", payload.WebhookHeaders)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"callId":"call_123","status":"initiated"}`))
	}))
	defer server.Close()

	c := New(server.URL, "secret-key", server.Client())
	data, err := c.CreateCall(context.Background(), CreateCallRequest{
		PhoneNumber:   "+14155550123",
		CallObjective: "Schedule a demo",
		FromName:      "Sarah",
		MaxRetries:    0,
		WebhookURL:    "https://example.com/webhooks/vocal",
		WebhookHeaders: map[string]string{
			"Authorization": "Bearer webhook-secret",
		},
	}, "idem-123")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret-key" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotIdempotency != "idem-123" {
		t.Fatalf("idempotency = %q", gotIdempotency)
	}
	payload := data.(map[string]any)
	if payload["callId"] != "call_123" {
		t.Fatalf("callId = %#v", payload["callId"])
	}
}

func TestCreateAgentSignupSendsIdempotencyAndNoAuth(t *testing.T) {
	var gotAuth, gotIdempotency string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotIdempotency = r.Header.Get("Idempotency-Key")
		if r.URL.Path != "/agent-signups" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var payload AgentSignupRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.AgentEmail != "agent@example.com" || payload.OwnerEmail != "owner@example.com" {
			t.Fatalf("payload = %#v", payload)
		}
		if payload.IdempotencyKey != "idem-agent-123" {
			t.Fatalf("payload idempotency = %q", payload.IdempotencyKey)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"req_123","apiKey":"sk_agent_secret","freeCallLimit":3}`))
	}))
	defer server.Close()

	c := New(server.URL, "", server.Client())
	data, err := c.CreateAgentSignup(context.Background(), AgentSignupRequest{
		AgentEmail:     "agent@example.com",
		OwnerEmail:     "owner@example.com",
		IdempotencyKey: "idem-agent-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotIdempotency != "idem-agent-123" {
		t.Fatalf("idempotency = %q", gotIdempotency)
	}
	payload := data.(map[string]any)
	if payload["id"] != "req_123" {
		t.Fatalf("id = %#v", payload["id"])
	}
}

func TestAPIErrorDecodesMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Invalid API key"}`))
	}))
	defer server.Close()

	c := New(server.URL, "bad-key", server.Client())
	_, err := c.ListCalls(context.Background(), "", 1, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Message != "Invalid API key" {
		t.Fatalf("api error = %#v", apiErr)
	}
}
