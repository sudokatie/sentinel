package checker

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/katieblackabee/sentinel/internal/storage"
)

type Scheduler struct {
	storage storage.Storage
	alerter Alerter
	config  SchedulerConfig

	checks      map[int64]*scheduledCheck
	mu          sync.RWMutex
	stopChan    chan struct{}
	wg          sync.WaitGroup
	cleanupStop chan struct{}
}

type SchedulerConfig struct {
	ConsecutiveFailures int
	RetentionDays       int
}

type scheduledCheck struct {
	check  *storage.Check
	ticker *time.Ticker
	stop   chan struct{}
}

func NewScheduler(store storage.Storage, alerter Alerter, config SchedulerConfig) *Scheduler {
	if config.ConsecutiveFailures < 1 {
		config.ConsecutiveFailures = 2
	}

	return &Scheduler{
		storage:     store,
		alerter:     alerter,
		config:      config,
		checks:      make(map[int64]*scheduledCheck),
		stopChan:    make(chan struct{}),
		cleanupStop: make(chan struct{}),
	}
}

func (s *Scheduler) Start() error {
	// Load all enabled checks
	checks, err := s.storage.ListEnabledChecks()
	if err != nil {
		return fmt.Errorf("loading checks: %w", err)
	}

	for _, check := range checks {
		if err := s.scheduleCheck(check); err != nil {
			fmt.Printf("failed to schedule check %s: %v\n", check.Name, err)
		}
	}

	// Start daily cleanup job
	go s.runCleanupJob()

	fmt.Printf("Scheduler started with %d checks\n", len(s.checks))
	return nil
}

func (s *Scheduler) runCleanupJob() {
	// Run cleanup once on startup
	s.doCleanup()

	// Then run daily at midnight
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.doCleanup()
		case <-s.cleanupStop:
			return
		case <-s.stopChan:
			return
		}
	}
}

func (s *Scheduler) doCleanup() {
	retentionDays := s.config.RetentionDays
	if retentionDays < 1 {
		retentionDays = 7 // Default 7 days
	}

	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	if err := s.storage.CleanupOldResults(cutoff); err != nil {
		fmt.Printf("cleanup error: %v\n", err)
	} else {
		fmt.Printf("Cleaned up results older than %d days\n", retentionDays)
	}
}

func (s *Scheduler) Stop() {
	close(s.stopChan)

	s.mu.Lock()
	for _, sc := range s.checks {
		close(sc.stop)
	}
	s.mu.Unlock()

	s.wg.Wait()
	fmt.Println("Scheduler stopped")
}

func (s *Scheduler) scheduleCheck(check *storage.Check) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Don't schedule if already exists
	if _, exists := s.checks[check.ID]; exists {
		return nil
	}

	interval := time.Duration(check.IntervalSecs) * time.Second
	if interval < time.Second {
		interval = time.Minute // Minimum 1 second, default 1 minute
	}

	sc := &scheduledCheck{
		check:  check,
		ticker: time.NewTicker(interval),
		stop:   make(chan struct{}),
	}

	s.checks[check.ID] = sc

	s.wg.Add(1)
	go s.runCheck(sc)

	return nil
}

func (s *Scheduler) runCheck(sc *scheduledCheck) {
	defer s.wg.Done()

	checker := NewHTTPChecker()

	// Add small jitter to prevent thundering herd
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
	time.Sleep(jitter)

	// Run immediately on start
	s.executeCheck(sc.check, checker)

	for {
		select {
		case <-sc.ticker.C:
			s.executeCheck(sc.check, checker)
		case <-sc.stop:
			sc.ticker.Stop()
			return
		case <-s.stopChan:
			sc.ticker.Stop()
			return
		}
	}
}

func (s *Scheduler) executeCheck(check *storage.Check, checker *HTTPChecker) {
	// Reload check from storage to get latest status
	current, err := s.storage.GetCheck(check.ID)
	if err != nil || current == nil {
		fmt.Printf("check %s no longer exists\n", check.Name)
		return
	}

	// Get previous result to determine current status
	lastResult, _ := s.storage.GetLatestResult(check.ID)
	if lastResult != nil {
		current.Status = lastResult.Status
		current.LastResponseMs = lastResult.ResponseTimeMs
		current.LastCheckedAt = &lastResult.CheckedAt
	} else {
		current.Status = "pending"
	}

	// Execute the check
	response := checker.Execute(&CheckRequest{
		URL:            current.URL,
		Timeout:        time.Duration(current.TimeoutSecs) * time.Second,
		ExpectedStatus: current.ExpectedStatus,
	})

	// Process the result
	if err := ProcessResult(s.storage, s.alerter, current, response, s.config.ConsecutiveFailures); err != nil {
		fmt.Printf("error processing result for %s: %v\n", current.Name, err)
	}
}

func (s *Scheduler) AddCheck(check *storage.Check) error {
	return s.scheduleCheck(check)
}

func (s *Scheduler) RemoveCheck(checkID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sc, exists := s.checks[checkID]; exists {
		close(sc.stop)
		delete(s.checks, checkID)
	}
}

func (s *Scheduler) UpdateCheck(check *storage.Check) error {
	// Remove old scheduling
	s.RemoveCheck(check.ID)

	// Re-schedule with new settings
	if check.Enabled {
		return s.scheduleCheck(check)
	}
	return nil
}

func (s *Scheduler) TriggerCheck(checkID int64) (*CheckResponse, error) {
	check, err := s.storage.GetCheck(checkID)
	if err != nil {
		return nil, fmt.Errorf("getting check: %w", err)
	}
	if check == nil {
		return nil, fmt.Errorf("check not found")
	}

	// Get current status
	lastResult, _ := s.storage.GetLatestResult(check.ID)
	if lastResult != nil {
		check.Status = lastResult.Status
	} else {
		check.Status = "pending"
	}

	checker := NewHTTPChecker()
	response := checker.Execute(&CheckRequest{
		URL:            check.URL,
		Timeout:        time.Duration(check.TimeoutSecs) * time.Second,
		ExpectedStatus: check.ExpectedStatus,
	})

	// Process and save the result
	if err := ProcessResult(s.storage, s.alerter, check, response, s.config.ConsecutiveFailures); err != nil {
		return nil, fmt.Errorf("processing result: %w", err)
	}

	return response, nil
}

func (s *Scheduler) ReloadChecks() error {
	// Stop all current checks
	s.mu.Lock()
	for _, sc := range s.checks {
		close(sc.stop)
	}
	s.checks = make(map[int64]*scheduledCheck)
	s.mu.Unlock()

	// Wait for all goroutines to stop
	s.wg.Wait()

	// Reload
	return s.Start()
}

func (s *Scheduler) GetCheckCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.checks)
}
