package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// ProbeAgent is a remote monitoring agent that registers with a Sentinel server,
// polls for checks to execute, and reports results.
type ProbeAgent struct {
	ServerURL    string
	ProbeKey     string
	LocationName string
	Region       string
	City         string
	Country      string
	Latitude     float64
	Longitude    float64
	httpClient   *http.Client
	probeID      int64
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
}

// CheckAssignment represents a check assigned to this probe.
type CheckAssignment struct {
	ID             int64         `json:"id"`
	URL            string        `json:"url"`
	Timeout        time.Duration `json:"timeout"`
	ExpectedStatus int           `json:"expected_status"`
}

// CheckResult represents the result of executing a check.
type CheckResult struct {
	CheckID        int64  `json:"check_id"`
	Status         string `json:"status"`
	StatusCode     int    `json:"status_code"`
	ResponseTimeMs int    `json:"response_time_ms"`
	Error          string `json:"error,omitempty"`
}

// RegistrationRequest is sent when registering with the server.
type RegistrationRequest struct {
	Name      string  `json:"name"`
	Region    string  `json:"region"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	APIKey    string  `json:"api_key"`
}

// RegistrationResponse is received after successful registration.
type RegistrationResponse struct {
	ID            int64         `json:"id"`
	CheckInterval time.Duration `json:"check_interval"`
}

// PollResponse contains checks assigned to this probe.
type PollResponse struct {
	Checks        []CheckAssignment `json:"checks"`
	CheckInterval time.Duration     `json:"check_interval"`
}

// NewProbeAgent creates a new probe agent with the given configuration.
func NewProbeAgent(serverURL, probeKey, locationName, region, city, country string, lat, lon float64) *ProbeAgent {
	return &ProbeAgent{
		ServerURL:    serverURL,
		ProbeKey:     probeKey,
		LocationName: locationName,
		Region:       region,
		City:         city,
		Country:      country,
		Latitude:     lat,
		Longitude:    lon,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Start registers with the server and begins the heartbeat and check polling loops.
func (a *ProbeAgent) Start(ctx context.Context) error {
	if err := a.registerWithRetry(ctx); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	log.Printf("Probe registered with ID %d", a.probeID)

	// Start heartbeat goroutine
	a.wg.Add(1)
	go a.heartbeatLoop(ctx)

	// Start check polling loop
	a.wg.Add(1)
	go a.pollChecksLoop(ctx)

	// Block until context cancelled or stop called
	select {
	case <-ctx.Done():
	case <-a.stopCh:
	}

	return nil
}

// Stop signals the agent to stop and deregisters from the server.
func (a *ProbeAgent) Stop() {
	close(a.stopCh)
	a.wg.Wait()

	if err := a.deregister(); err != nil {
		log.Printf("Deregistration failed: %v", err)
	} else {
		log.Printf("Probe deregistered")
	}
}

// ProbeID returns the registered probe ID.
func (a *ProbeAgent) ProbeID() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.probeID
}

func (a *ProbeAgent) registerWithRetry(ctx context.Context) error {
	const maxRetries = 5
	backoff := time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := a.register()
		if err == nil {
			return nil
		}

		log.Printf("Registration attempt %d failed: %v", attempt, err)

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-a.stopCh:
				return fmt.Errorf("stopped during registration")
			case <-time.After(backoff):
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
		}
	}

	return fmt.Errorf("max registration retries exceeded")
}

func (a *ProbeAgent) register() error {
	req := RegistrationRequest{
		Name:      a.LocationName,
		Region:    a.Region,
		City:      a.City,
		Country:   a.Country,
		Latitude:  a.Latitude,
		Longitude: a.Longitude,
		APIKey:    a.ProbeKey,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Post(
		a.ServerURL+"/api/probes/register",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var regResp RegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return err
	}

	a.mu.Lock()
	a.probeID = regResp.ID
	a.mu.Unlock()

	return nil
}

func (a *ProbeAgent) deregister() error {
	probeID := a.ProbeID()
	if probeID == 0 {
		return nil
	}

	req, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("%s/api/probes/%d", a.ServerURL, probeID),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregistration failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (a *ProbeAgent) heartbeatLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.sendHeartbeat(); err != nil {
				log.Printf("Heartbeat failed: %v", err)
			}
		}
	}
}

func (a *ProbeAgent) sendHeartbeat() error {
	probeID := a.ProbeID()
	if probeID == 0 {
		return fmt.Errorf("not registered")
	}

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/api/probes/%d/heartbeat", a.ServerURL, probeID),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (a *ProbeAgent) pollChecksLoop(ctx context.Context) {
	defer a.wg.Done()

	checkInterval := 30 * time.Second
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Run immediately on start
	a.pollAndExecuteChecks()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.pollAndExecuteChecks()
		}
	}
}

func (a *ProbeAgent) pollAndExecuteChecks() {
	checks, err := a.pollChecks()
	if err != nil {
		log.Printf("Poll checks failed: %v", err)
		return
	}

	for _, check := range checks {
		a.executeCheck(check)
	}
}

func (a *ProbeAgent) pollChecks() ([]CheckAssignment, error) {
	probeID := a.ProbeID()
	if probeID == 0 {
		return nil, fmt.Errorf("not registered")
	}

	resp, err := a.httpClient.Get(fmt.Sprintf("%s/api/probes/checks", a.ServerURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("poll checks failed with status %d: %s", resp.StatusCode, string(body))
	}

	var pollResp PollResponse
	if err := json.NewDecoder(resp.Body).Decode(&pollResp); err != nil {
		return nil, err
	}

	return pollResp.Checks, nil
}

func (a *ProbeAgent) executeCheck(check CheckAssignment) {
	timeout := check.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	status, statusCode, responseTimeMs, err := a.runCheck(context.Background(), check.URL, timeout)

	result := CheckResult{
		CheckID:        check.ID,
		Status:         status,
		StatusCode:     statusCode,
		ResponseTimeMs: responseTimeMs,
	}
	if err != nil {
		result.Error = err.Error()
	}

	if err := a.submitResult(result); err != nil {
		log.Printf("Failed to submit result for check %d: %v", check.ID, err)
	}
}

func (a *ProbeAgent) runCheck(ctx context.Context, checkURL string, timeout time.Duration) (status string, statusCode int, responseTimeMs int, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return "error", 0, 0, err
	}

	req.Header.Set("User-Agent", "Sentinel-Probe/1.0")

	start := time.Now()
	resp, err := a.httpClient.Do(req)
	elapsed := time.Since(start)

	responseTimeMs = int(elapsed.Milliseconds())

	if err != nil {
		return "error", 0, responseTimeMs, err
	}
	defer resp.Body.Close()

	statusCode = resp.StatusCode
	if statusCode >= 200 && statusCode < 400 {
		status = "up"
	} else {
		status = "down"
	}

	return status, statusCode, responseTimeMs, nil
}

func (a *ProbeAgent) submitResult(result CheckResult) error {
	probeID := a.ProbeID()
	if probeID == 0 {
		return fmt.Errorf("not registered")
	}

	body, err := json.Marshal(result)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Post(
		fmt.Sprintf("%s/api/probes/%d/results", a.ServerURL, probeID),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("submit result failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
