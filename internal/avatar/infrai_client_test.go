package avatar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientReadsBusinessErrorBeforeHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/auth/user/update/user-42" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "request-42" {
			t.Fatalf("headers = %v", r.Header)
		}
		var body struct {
			UserID   string         `json:"user_id"`
			Metadata map[string]any `json:"metadata"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.UserID != "user-42" || body.Metadata["avatar_image_id"] != "image-7" {
			t.Fatalf("body = %#v", body)
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": false, "error": map[string]string{"code": "INVALID_ARGUMENT", "message": "invalid metadata"},
		})
	}))
	defer server.Close()

	client := NewInfraiClient(server.URL, "test-key", server.Client())
	err := client.UpdateUser(context.Background(), "user-42", map[string]any{"avatar_image_id": "image-7"}, "request-42")
	infraiErr, ok := err.(*InfraiError)
	if !ok || infraiErr.Code != "INVALID_ARGUMENT" || infraiErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("error = %#v", err)
	}
}

func TestClientRetriesRateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{"id": "pending"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{"id": "image-7"}})
	}))
	defer server.Close()

	client := NewInfraiClient(server.URL, "test-key", server.Client())
	client.sleep = func(context.Context, time.Duration) error { return nil }
	id, err := client.SmartCrop(context.Background(), "image-6", "1:1", "request-42")
	if err != nil || id != "image-7" || attempts != 2 {
		t.Fatalf("id = %q, attempts = %d, error = %v", id, attempts, err)
	}
}
