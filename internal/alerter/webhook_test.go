package alerter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/katieblackabee/sentinel/internal/config"
	"github.com/katieblackabee/sentinel/internal/storage"
)

func TestSlackSender_Send(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSlackSender(&config.SlackConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	})

	alert := &Alert{
		Type: "down",
		Check: &storage.Check{
			Name: "Test Service",
			URL:  "https://example.com",
		},
		Error:     "Connection refused",
		Timestamp: time.Now(),
	}

	err := sender.Send(alert)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var msg SlackMessage
	if err := json.Unmarshal(receivedBody, &msg); err != nil {
		t.Fatalf("failed to unmarshal slack message: %v", err)
	}

	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}

	if msg.Attachments[0].Color != "danger" {
		t.Errorf("expected color danger, got %s", msg.Attachments[0].Color)
	}
}

func TestSlackSender_SendRecovery(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSlackSender(&config.SlackConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	})

	alert := &Alert{
		Type: "recovery",
		Check: &storage.Check{
			Name: "Test Service",
			URL:  "https://example.com",
		},
		Timestamp: time.Now(),
	}

	err := sender.Send(alert)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var msg SlackMessage
	if err := json.Unmarshal(receivedBody, &msg); err != nil {
		t.Fatalf("failed to unmarshal slack message: %v", err)
	}

	if msg.Attachments[0].Color != "good" {
		t.Errorf("expected color good, got %s", msg.Attachments[0].Color)
	}
}

func TestSlackSender_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sender := NewSlackSender(&config.SlackConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	})

	alert := &Alert{
		Type: "down",
		Check: &storage.Check{
			Name: "Test Service",
			URL:  "https://example.com",
		},
		Error:     "Connection refused",
		Timestamp: time.Now(),
	}

	err := sender.Send(alert)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestDiscordSender_Send(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender := NewDiscordSender(&config.DiscordConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	})

	alert := &Alert{
		Type: "down",
		Check: &storage.Check{
			Name: "Test Service",
			URL:  "https://example.com",
		},
		Error:     "Connection refused",
		Timestamp: time.Now(),
	}

	err := sender.Send(alert)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var msg DiscordMessage
	if err := json.Unmarshal(receivedBody, &msg); err != nil {
		t.Fatalf("failed to unmarshal discord message: %v", err)
	}

	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(msg.Embeds))
	}

	// Red color for down alerts
	if msg.Embeds[0].Color != 15158332 {
		t.Errorf("expected color 15158332, got %d", msg.Embeds[0].Color)
	}
}

func TestDiscordSender_SendRecovery(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender := NewDiscordSender(&config.DiscordConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	})

	alert := &Alert{
		Type: "recovery",
		Check: &storage.Check{
			Name: "Test Service",
			URL:  "https://example.com",
		},
		Timestamp: time.Now(),
	}

	err := sender.Send(alert)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var msg DiscordMessage
	if err := json.Unmarshal(receivedBody, &msg); err != nil {
		t.Fatalf("failed to unmarshal discord message: %v", err)
	}

	// Green color for recovery
	if msg.Embeds[0].Color != 3066993 {
		t.Errorf("expected color 3066993, got %d", msg.Embeds[0].Color)
	}
}

func TestDiscordSender_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sender := NewDiscordSender(&config.DiscordConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	})

	alert := &Alert{
		Type: "down",
		Check: &storage.Check{
			Name: "Test Service",
			URL:  "https://example.com",
		},
		Error:     "Connection refused",
		Timestamp: time.Now(),
	}

	err := sender.Send(alert)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSlackSender_SSLExpiryAlert(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSlackSender(&config.SlackConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	})

	alert := &Alert{
		Type: "ssl_expiry",
		Check: &storage.Check{
			Name: "Secure API",
			URL:  "https://api.example.com",
		},
		Error:     "SSL certificate expires in 15 days (on Mar 1, 2026)",
		Timestamp: time.Now(),
	}

	err := sender.Send(alert)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var msg SlackMessage
	if err := json.Unmarshal(receivedBody, &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if msg.Attachments[0].Color != "warning" {
		t.Errorf("expected color warning, got %s", msg.Attachments[0].Color)
	}
	if msg.Attachments[0].Title != "⚠️ SSL EXPIRING: Secure API" {
		t.Errorf("unexpected title: %s", msg.Attachments[0].Title)
	}
}

func TestDiscordSender_SSLExpiryAlert(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender := NewDiscordSender(&config.DiscordConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	})

	alert := &Alert{
		Type: "ssl_expiry",
		Check: &storage.Check{
			Name: "Secure API",
			URL:  "https://api.example.com",
		},
		Error:     "SSL certificate expires in 15 days",
		Timestamp: time.Now(),
	}

	err := sender.Send(alert)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var msg DiscordMessage
	if err := json.Unmarshal(receivedBody, &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Yellow color for warning
	if msg.Embeds[0].Color != 16776960 {
		t.Errorf("expected color 16776960, got %d", msg.Embeds[0].Color)
	}
}
