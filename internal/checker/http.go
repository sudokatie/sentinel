package checker

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"
)

type HTTPChecker struct {
	client     *http.Client
	RetryDelay time.Duration
}

type CheckRequest struct {
	URL            string
	Timeout        time.Duration
	ExpectedStatus int
}

type CheckResponse struct {
	StatusCode     int
	ResponseTimeMs int
	Error          error
}

func NewHTTPChecker() *HTTPChecker {
	return NewHTTPCheckerWithRetry(5 * time.Second)
}

func NewHTTPCheckerWithRetry(retryDelay time.Duration) *HTTPChecker {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	return &HTTPChecker{client: client, RetryDelay: retryDelay}
}

func (h *HTTPChecker) Execute(req *CheckRequest) *CheckResponse {
	response := h.doRequest(req)

	// Retry once after delay on failure (per spec: 1 retry after 5 seconds)
	if !response.IsSuccess(req.ExpectedStatus) && h.RetryDelay > 0 {
		time.Sleep(h.RetryDelay)
		response = h.doRequest(req)
	}

	return response
}

func (h *HTTPChecker) doRequest(req *CheckRequest) *CheckResponse {
	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", req.URL, nil)
	if err != nil {
		return &CheckResponse{Error: err}
	}

	httpReq.Header.Set("User-Agent", "Sentinel/1.0 (Uptime Monitor)")

	start := time.Now()
	resp, err := h.client.Do(httpReq)
	elapsed := time.Since(start)

	response := &CheckResponse{
		ResponseTimeMs: int(elapsed.Milliseconds()),
	}

	if err != nil {
		response.Error = err
		return response
	}
	defer resp.Body.Close()

	response.StatusCode = resp.StatusCode
	return response
}

func (r *CheckResponse) IsSuccess(expectedStatus int) bool {
	if r.Error != nil {
		return false
	}
	if expectedStatus == 0 {
		expectedStatus = 200
	}
	return r.StatusCode == expectedStatus
}
