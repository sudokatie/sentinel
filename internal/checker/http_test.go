package checker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPCheckerSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewHTTPChecker()
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

			checker := NewHTTPChecker()
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

	checker := NewHTTPChecker()
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
	checker := NewHTTPChecker()
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
	checker := NewHTTPChecker()
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

	checker := NewHTTPChecker()
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

	checker := NewHTTPChecker()
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

	checker := NewHTTPChecker()
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
