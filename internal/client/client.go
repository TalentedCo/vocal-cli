package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("VOCAL API returned HTTP %d", e.StatusCode)
}

type CreateCallRequest struct {
	ExternalID              string            `json:"externalId,omitempty"`
	PhoneNumber             string            `json:"phoneNumber"`
	CallObjective           string            `json:"callObjective"`
	FromName                string            `json:"fromName"`
	RecipientName           string            `json:"recipientName,omitempty"`
	Voice                   string            `json:"voice,omitempty"`
	MaxRetries              int               `json:"maxRetries"`
	SkipObjectiveValidation bool              `json:"skipObjectiveValidation,omitempty"`
	WebhookURL              string            `json:"webhookUrl,omitempty"`
	WebhookHeaders          map[string]string `json:"webhookHeaders,omitempty"`
}

func New(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		HTTPClient: httpClient,
	}
}

func (c *Client) CreateCall(ctx context.Context, request CreateCallRequest, idempotencyKey string) (any, error) {
	return c.do(ctx, http.MethodPost, "/calls", request, idempotencyKey)
}

func (c *Client) GetCall(ctx context.Context, callID string) (any, error) {
	return c.do(ctx, http.MethodGet, "/calls/"+url.PathEscape(callID), nil, "")
}

func (c *Client) ListCalls(ctx context.Context, status string, limit, offset int) (any, error) {
	values := url.Values{}
	if status != "" {
		values.Set("status", status)
	}
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		values.Set("offset", fmt.Sprintf("%d", offset))
	}
	path := "/calls"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return c.do(ctx, http.MethodGet, path, nil, "")
}

func (c *Client) StreamCall(ctx context.Context, callID string) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/calls/"+url.PathEscape(callID)+"/stream", nil, "")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		apiErr, err := decodeAPIError(resp)
		if err != nil {
			return nil, err
		}
		return nil, apiErr
	}
	return resp, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, idempotencyKey string) (any, error) {
	req, err := c.newRequest(ctx, method, path, body, idempotencyKey)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr, err := decodeAPIError(resp)
		if err != nil {
			return nil, err
		}
		return nil, apiErr
	}

	var data any
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		if err == io.EOF {
			return map[string]any{}, nil
		}
		return nil, err
	}
	return data, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any, idempotencyKey string) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		content, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(content)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return req, nil
}

func decodeAPIError(resp *http.Response) (*APIError, error) {
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	message := strings.TrimSpace(string(content))
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err == nil {
		if value, ok := payload["error"].(string); ok && value != "" {
			message = value
		} else if value, ok := payload["message"].(string); ok && value != "" {
			message = value
		}
	}
	return &APIError{StatusCode: resp.StatusCode, Message: message, Body: string(content)}, nil
}
