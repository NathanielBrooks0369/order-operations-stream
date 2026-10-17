package infrai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateChannelRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	var calls int
	var firstKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/realtime/channel/create" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if calls == 1 {
			firstKey = r.Header.Get("Idempotency-Key")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"","message":"retry later"},"metadata":{}}`))
			return
		}
		if r.Header.Get("Idempotency-Key") != firstKey {
			t.Fatal("retry changed idempotency key")
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := New("test-key")
	client.baseURL = server.URL
	client.sleep = func(time.Duration) {}
	if err := client.CreateChannel(context.Background(), "store-operations"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}
