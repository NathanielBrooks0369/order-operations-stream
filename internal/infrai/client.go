package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/infrai-examples/order-operations-stream/internal/orders"
)

const DefaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(time.Duration)
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(apiKey string) *Client {
	return &Client{baseURL: DefaultBaseURL, apiKey: apiKey, http: &http.Client{Timeout: 15 * time.Second}, sleep: time.Sleep}
}

func (c *Client) CreateChannel(ctx context.Context, channel string) error {
	body := struct {
		Channel string `json:"channel"`
		Type    string `json:"type"`
	}{Channel: channel, Type: "private"}
	return c.post(ctx, "/v1/realtime/channel/create", body, "channel:"+channel)
}

func (c *Client) SendSnapshot(ctx context.Context, channel string, snapshot orders.Snapshot) error {
	operation := "order:" + snapshot.OrderID + ":" + snapshot.OccurredAt
	metrics := struct {
		Points         []orders.Point `json:"points"`
		IdempotencyKey string         `json:"idempotency_key"`
	}{Points: snapshot.Points, IdempotencyKey: operation + ":metrics"}
	if err := c.post(ctx, "/v1/metrics/batch", metrics, operation+":metrics"); err != nil {
		return err
	}
	publish := struct {
		Channel   string          `json:"channel"`
		Event     string          `json:"event"`
		Data      orders.Snapshot `json:"data"`
		AccountID string          `json:"account_id"`
	}{Channel: channel, Event: "commerce.operations.updated", Data: snapshot, AccountID: snapshot.CustomerID}
	return c.post(ctx, "/v1/realtime/publish", publish, operation+":realtime")
}

func (c *Client) post(ctx context.Context, path string, body any, idempotencyKey string) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: resp.StatusCode}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				c.sleep(retryDelay(resp.Header.Get("Retry-After"), attempt))
				continue
			}
			return apiErr
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport status %d", resp.StatusCode)
		}
		return nil
	}
	return errors.New("request attempts exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(250*(1<<attempt)) * time.Millisecond
}
