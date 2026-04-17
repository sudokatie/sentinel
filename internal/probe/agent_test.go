package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewProbeAgent(t *testing.T) {
	agent := NewProbeAgent(
		"http://localhost:8080",
		"test-key",
		"NYC-Probe",
		"US-East",
		"New York",
		"USA",
		40.7128,
		-74.0060,
	)

	if agent.ServerURL != "http://localhost:8080" {
		t.Errorf("ServerURL = %v, expected http://localhost:8080", agent.ServerURL)
	}
	if agent.ProbeKey != "test-key" {
		t.Errorf("ProbeKey = %v, expected test-key", agent.ProbeKey)
	}
	if agent.LocationName != "NYC-Probe" {
		t.Errorf("LocationName = %v, expected NYC-Probe", agent.LocationName)
	}
	if agent.Region != "US-East" {
		t.Errorf("Region = %v, expected US-East", agent.Region)
	}
	if agent.City != "New York" {
		t.Errorf("City = %v, expected New York", agent.City)
	}
	if agent.Country != "USA" {
		t.Errorf("Country = %v, expected USA", agent.Country)
	}
	if agent.Latitude != 40.7128 {
		t.Errorf("Latitude = %v, expected 40.7128", agent.Latitude)
	}
	if agent.Longitude != -74.0060 {
		t.Errorf("Longitude = %v, expected -74.0060", agent.Longitude)
	}
	if agent.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if agent.stopCh == nil {
		t.Error("stopCh is nil")
	}
}

func TestAgentRegistration(t *testing.T) {
	var receivedReq RegistrationRequest
	var registrationCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/probes/register" && r.Method == http.MethodPost {
			registrationCalled = true

			if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
				t.Errorf("Failed to decode request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			resp := RegistrationResponse{
				ID:            42,
				CheckInterval: 30 * time.Second,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	agent := NewProbeAgent(
		server.URL,
		"test-api-key",
		"Test-Probe",
		"US-West",
		"San Francisco",
		"USA",
		37.7749,
		-122.4194,
	)

	err := agent.register()
	if err != nil {
		t.Fatalf("register() failed: %v", err)
	}

	if !registrationCalled {
		t.Error("Registration endpoint was not called")
	}

	if receivedReq.Name != "Test-Probe" {
		t.Errorf("receivedReq.Name = %v, expected Test-Probe", receivedReq.Name)
	}
	if receivedReq.Region != "US-West" {
		t.Errorf("receivedReq.Region = %v, expected US-West", receivedReq.Region)
	}
	if receivedReq.City != "San Francisco" {
		t.Errorf("receivedReq.City = %v, expected San Francisco", receivedReq.City)
	}
	if receivedReq.Country != "USA" {
		t.Errorf("receivedReq.Country = %v, expected USA", receivedReq.Country)
	}
	if receivedReq.Latitude != 37.7749 {
		t.Errorf("receivedReq.Latitude = %v, expected 37.7749", receivedReq.Latitude)
	}
	if receivedReq.Longitude != -122.4194 {
		t.Errorf("receivedReq.Longitude = %v, expected -122.4194", receivedReq.Longitude)
	}
	if receivedReq.APIKey != "test-api-key" {
		t.Errorf("receivedReq.APIKey = %v, expected test-api-key", receivedReq.APIKey)
	}

	if agent.ProbeID() != 42 {
		t.Errorf("ProbeID() = %v, expected 42", agent.ProbeID())
	}
}

func TestAgentHeartbeat(t *testing.T) {
	var heartbeatCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/probes/register" && r.Method == http.MethodPost:
			resp := RegistrationResponse{ID: 1}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/probes/1/heartbeat" && r.Method == http.MethodPost:
			atomic.AddInt32(&heartbeatCount, 1)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	agent := NewProbeAgent(
		server.URL,
		"key",
		"test",
		"region",
		"city",
		"country",
		0, 0,
	)

	// Register first
	if err := agent.register(); err != nil {
		t.Fatalf("register() failed: %v", err)
	}

	// Test sendHeartbeat directly
	if err := agent.sendHeartbeat(); err != nil {
		t.Errorf("sendHeartbeat() failed: %v", err)
	}

	count := atomic.LoadInt32(&heartbeatCount)
	if count != 1 {
		t.Errorf("heartbeatCount = %v, expected 1", count)
	}

	// Send another heartbeat
	if err := agent.sendHeartbeat(); err != nil {
		t.Errorf("second sendHeartbeat() failed: %v", err)
	}

	count = atomic.LoadInt32(&heartbeatCount)
	if count != 2 {
		t.Errorf("heartbeatCount = %v, expected 2", count)
	}
}

func TestAgentCheckExecution(t *testing.T) {
	// Create a mock target server that we'll check
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer targetServer.Close()

	var receivedResult CheckResult
	var resultSubmitted bool

	// Create the Sentinel server mock
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/probes/register" && r.Method == http.MethodPost:
			resp := RegistrationResponse{ID: 5}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/probes/checks" && r.Method == http.MethodGet:
			resp := PollResponse{
				Checks: []CheckAssignment{
					{
						ID:             100,
						URL:            targetServer.URL,
						Timeout:        5 * time.Second,
						ExpectedStatus: 200,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/probes/5/results" && r.Method == http.MethodPost:
			resultSubmitted = true
			json.NewDecoder(r.Body).Decode(&receivedResult)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	agent := NewProbeAgent(server.URL, "key", "test", "region", "city", "country", 0, 0)

	// Register
	if err := agent.register(); err != nil {
		t.Fatalf("register() failed: %v", err)
	}

	// Poll and execute checks
	agent.pollAndExecuteChecks()

	if !resultSubmitted {
		t.Error("Result was not submitted")
	}

	if receivedResult.CheckID != 100 {
		t.Errorf("receivedResult.CheckID = %v, expected 100", receivedResult.CheckID)
	}
	if receivedResult.Status != "up" {
		t.Errorf("receivedResult.Status = %v, expected up", receivedResult.Status)
	}
	if receivedResult.StatusCode != 200 {
		t.Errorf("receivedResult.StatusCode = %v, expected 200", receivedResult.StatusCode)
	}
	if receivedResult.ResponseTimeMs < 0 {
		t.Errorf("receivedResult.ResponseTimeMs = %v, expected >= 0", receivedResult.ResponseTimeMs)
	}
	if receivedResult.Error != "" {
		t.Errorf("receivedResult.Error = %v, expected empty", receivedResult.Error)
	}
}

func TestAgentCheckExecutionError(t *testing.T) {
	var receivedResult CheckResult

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/probes/register" && r.Method == http.MethodPost:
			resp := RegistrationResponse{ID: 6}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/probes/checks" && r.Method == http.MethodGet:
			resp := PollResponse{
				Checks: []CheckAssignment{
					{
						ID:      101,
						URL:     "http://localhost:59999", // Non-existent server
						Timeout: 1 * time.Second,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/probes/6/results" && r.Method == http.MethodPost:
			json.NewDecoder(r.Body).Decode(&receivedResult)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	agent := NewProbeAgent(server.URL, "key", "test", "region", "city", "country", 0, 0)
	agent.register()

	agent.pollAndExecuteChecks()

	if receivedResult.Status != "error" {
		t.Errorf("receivedResult.Status = %v, expected error", receivedResult.Status)
	}
	if receivedResult.Error == "" {
		t.Error("receivedResult.Error should not be empty for connection error")
	}
}

func TestAgentGracefulShutdown(t *testing.T) {
	var deregisterCalled bool
	var deregisterProbeID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/probes/register" && r.Method == http.MethodPost:
			resp := RegistrationResponse{ID: 10}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/probes/10" && r.Method == http.MethodDelete:
			deregisterCalled = true
			deregisterProbeID = "10"
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/probes/checks" && r.Method == http.MethodGet:
			resp := PollResponse{Checks: []CheckAssignment{}}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/probes/10/heartbeat" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	agent := NewProbeAgent(server.URL, "key", "test", "region", "city", "country", 0, 0)

	ctx, cancel := context.WithCancel(context.Background())

	// Start agent in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- agent.Start(ctx)
	}()

	// Give it time to register and start loops
	time.Sleep(100 * time.Millisecond)

	// Verify it registered
	if agent.ProbeID() != 10 {
		t.Errorf("ProbeID() = %v, expected 10", agent.ProbeID())
	}

	// Stop the agent
	cancel()
	agent.Stop()

	// Verify deregister was called
	if !deregisterCalled {
		t.Error("Deregister was not called on shutdown")
	}
	if deregisterProbeID != "10" {
		t.Errorf("Deregister probe ID = %v, expected 10", deregisterProbeID)
	}
}

func TestAgentRunCheck(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		timeout        time.Duration
		expectedStatus string
		expectedCode   int
		expectError    bool
	}{
		{
			name: "successful check",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			timeout:        5 * time.Second,
			expectedStatus: "up",
			expectedCode:   200,
			expectError:    false,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			timeout:        5 * time.Second,
			expectedStatus: "down",
			expectedCode:   500,
			expectError:    false,
		},
		{
			name: "redirect",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusMovedPermanently)
			},
			timeout:        5 * time.Second,
			expectedStatus: "up", // 3xx responses are valid HTTP responses
			expectedCode:   301,
			expectError:    false,
		},
		{
			name: "slow response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(200 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			},
			timeout:        100 * time.Millisecond,
			expectedStatus: "error",
			expectedCode:   0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			agent := NewProbeAgent("", "", "", "", "", "", 0, 0)

			status, statusCode, responseTimeMs, err := agent.runCheck(context.Background(), server.URL, tt.timeout)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if status != tt.expectedStatus {
				t.Errorf("status = %v, expected %v", status, tt.expectedStatus)
			}
			if statusCode != tt.expectedCode {
				t.Errorf("statusCode = %v, expected %v", statusCode, tt.expectedCode)
			}
			if responseTimeMs < 0 {
				t.Errorf("responseTimeMs = %v, expected >= 0", responseTimeMs)
			}
		})
	}
}

func TestAgentRegistrationRetry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/probes/register" && r.Method == http.MethodPost {
			count := atomic.AddInt32(&attempts, 1)
			if count < 3 {
				// Fail the first 2 attempts
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// Succeed on the 3rd attempt
			resp := RegistrationResponse{ID: 99}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	agent := NewProbeAgent(server.URL, "key", "test", "region", "city", "country", 0, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := agent.registerWithRetry(ctx)
	if err != nil {
		t.Fatalf("registerWithRetry() failed: %v", err)
	}

	count := atomic.LoadInt32(&attempts)
	if count != 3 {
		t.Errorf("attempts = %v, expected 3", count)
	}

	if agent.ProbeID() != 99 {
		t.Errorf("ProbeID() = %v, expected 99", agent.ProbeID())
	}
}
