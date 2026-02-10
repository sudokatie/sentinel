package checker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestChecker creates a checker with no retry delay for faster tests
func newTestChecker() *HTTPChecker {
	return NewHTTPCheckerWithRetry(0)
}

func TestHTTPCheckerSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	resp := checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if resp.ResponseTimeMs < 0 {
		t.Error("expected non-negative response time")
	}
	if !resp.IsSuccess(200) {
		t.Error("expected IsSuccess to return true")
	}
}

func TestHTTPCheckerDifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name           string
		serverStatus   int
		expectedStatus int
		wantSuccess    bool
	}{
		{"200 OK", 200, 200, true},
		{"201 Created", 201, 201, true},
		{"204 No Content", 204, 204, true},
		{"200 but expect 201", 200, 201, false},
		{"404 Not Found", 404, 200, false},
		{"500 Server Error", 500, 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			checker := newTestChecker()
			resp := checker.Execute(&CheckRequest{
				URL:            server.URL,
				Timeout:        5 * time.Second,
				ExpectedStatus: tt.expectedStatus,
			})

			if resp.Error != nil {
				t.Fatalf("unexpected error: %v", resp.Error)
			}
			if resp.StatusCode != tt.serverStatus {
				t.Errorf("expected status %d, got %d", tt.serverStatus, resp.StatusCode)
			}
			if resp.IsSuccess(tt.expectedStatus) != tt.wantSuccess {
				t.Errorf("expected IsSuccess=%v, got %v", tt.wantSuccess, resp.IsSuccess(tt.expectedStatus))
			}
		})
	}
}

func TestHTTPCheckerTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	resp := checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        100 * time.Millisecond,
		ExpectedStatus: 200,
	})

	if resp.Error == nil {
		t.Error("expected timeout error")
	}
	if resp.IsSuccess(200) {
		t.Error("expected IsSuccess to return false on timeout")
	}
}

func TestHTTPCheckerInvalidURL(t *testing.T) {
	checker := newTestChecker()
	resp := checker.Execute(&CheckRequest{
		URL:            "not-a-valid-url",
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestHTTPCheckerConnectionRefused(t *testing.T) {
	checker := newTestChecker()
	resp := checker.Execute(&CheckRequest{
		URL:            "http://localhost:59999", // Unlikely to be in use
		Timeout:        2 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error == nil {
		t.Error("expected connection error")
	}
}

func TestHTTPCheckerRedirects(t *testing.T) {
	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redirectCount < 3 {
			redirectCount++
			http.Redirect(w, r, "/redirect", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	resp := checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 after redirects, got %d", resp.StatusCode)
	}
}

func TestHTTPCheckerResponseTime(t *testing.T) {
	delay := 100 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	resp := checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	// Response time should be at least the delay
	if resp.ResponseTimeMs < int(delay.Milliseconds()) {
		t.Errorf("expected response time >= %dms, got %dms", delay.Milliseconds(), resp.ResponseTimeMs)
	}
}

func TestHTTPCheckerUserAgent(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if receivedUA != "Sentinel/1.0 (Uptime Monitor)" {
		t.Errorf("unexpected User-Agent: %s", receivedUA)
	}
}

func TestIsSuccessDefaultStatus(t *testing.T) {
	resp := &CheckResponse{StatusCode: 200}

	// expectedStatus=0 should default to 200
	if !resp.IsSuccess(0) {
		t.Error("expected IsSuccess(0) to return true for status 200")
	}
}

func TestHTTPCheckerRetryOnFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call fails
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			// Second call succeeds
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Use short retry delay for testing
	checker := NewHTTPCheckerWithRetry(10 * time.Millisecond)
	resp := checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 after retry, got %d", resp.StatusCode)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (initial + retry), got %d", callCount)
	}
}

func TestHTTPCheckerNoRetryOnSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewHTTPCheckerWithRetry(10 * time.Millisecond)
	resp := checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if callCount != 1 {
		t.Errorf("expected only 1 call on success, got %d", callCount)
	}
}

func TestHTTPCheckerNoRetryWhenDisabled(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// RetryDelay of 0 disables retry
	checker := NewHTTPCheckerWithRetry(0)
	resp := checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("expected only 1 call when retry disabled, got %d", callCount)
	}
}

func TestHTTPCheckerSSLFieldsNilForHTTP(t *testing.T) {
	// HTTP (non-TLS) connections should have nil SSL fields
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := newTestChecker()
	resp := checker.Execute(&CheckRequest{
		URL:            server.URL,
		Timeout:        5 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.SSLExpiresAt != nil {
		t.Error("expected SSLExpiresAt to be nil for HTTP")
	}
	if resp.SSLDaysLeft != 0 {
		t.Error("expected SSLDaysLeft to be 0 for HTTP")
	}
	if resp.SSLIssuer != "" {
		t.Error("expected SSLIssuer to be empty for HTTP")
	}
}

func TestHTTPCheckerSSLFieldsForHTTPS(t *testing.T) {
	// Test against a real HTTPS URL (google.com should have valid cert)
	checker := newTestChecker()
	resp := checker.Execute(&CheckRequest{
		URL:            "https://www.google.com",
		Timeout:        10 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error != nil {
		t.Skipf("skipping HTTPS test (network unavailable): %v", resp.Error)
	}

	if resp.SSLExpiresAt == nil {
		t.Error("expected SSLExpiresAt to be set for HTTPS")
	}
	if resp.SSLDaysLeft < 0 {
		t.Error("expected SSLDaysLeft to be positive for valid cert")
	}
	if resp.SSLIssuer == "" {
		t.Error("expected SSLIssuer to be set for HTTPS")
	}
}
