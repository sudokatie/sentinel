package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	// Add busy_timeout to handle concurrent access gracefully
	// Wait up to 5 seconds for locks to clear before failing
	connStr := dbPath + "?_busy_timeout=5000"
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	s := &SQLiteStorage{db: db}
	if err := s.Migrate(); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

func (s *SQLiteStorage) Migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS checks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			interval_seconds INTEGER NOT NULL DEFAULT 3600,
			timeout_seconds INTEGER NOT NULL DEFAULT 10,
			expected_status INTEGER NOT NULL DEFAULT 200,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			tags TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS check_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			check_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			status_code INTEGER,
			response_time_ms INTEGER,
			error_message TEXT,
			checked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_check_results_check_id ON check_results(check_id)`,
		`CREATE INDEX IF NOT EXISTS idx_check_results_checked_at ON check_results(checked_at)`,
		`CREATE TABLE IF NOT EXISTS incidents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			check_id INTEGER NOT NULL,
			started_at DATETIME NOT NULL,
			ended_at DATETIME,
			duration_seconds INTEGER,
			cause TEXT,
			FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_incidents_check_id ON incidents(check_id)`,
		`CREATE TABLE IF NOT EXISTS alert_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			incident_id INTEGER,
			channel TEXT NOT NULL,
			sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			success BOOLEAN,
			error_message TEXT,
			FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS hourly_aggregates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			check_id INTEGER NOT NULL,
			hour DATETIME NOT NULL,
			total_checks INTEGER NOT NULL,
			success_count INTEGER NOT NULL,
			failure_count INTEGER NOT NULL,
			avg_response_ms INTEGER,
			min_response_ms INTEGER,
			max_response_ms INTEGER,
			uptime_percent REAL,
			FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE,
			UNIQUE(check_id, hour)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hourly_aggregates_check_id ON hourly_aggregates(check_id)`,
		`CREATE INDEX IF NOT EXISTS idx_hourly_aggregates_hour ON hourly_aggregates(hour)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}

	// Add SSL columns to check_results (will fail silently if columns exist)
	sslMigrations := []string{
		`ALTER TABLE check_results ADD COLUMN ssl_expires_at DATETIME`,
		`ALTER TABLE check_results ADD COLUMN ssl_days_left INTEGER`,
		`ALTER TABLE check_results ADD COLUMN ssl_issuer TEXT`,
	}
	for _, m := range sslMigrations {
		s.db.Exec(m) // Ignore errors (column already exists)
	}

	return nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// Checks

func (s *SQLiteStorage) CreateCheck(check *Check) error {
	tagsJSON, err := json.Marshal(check.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	result, err := s.db.Exec(`
		INSERT INTO checks (name, url, interval_seconds, timeout_seconds, expected_status, enabled, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, check.Name, check.URL, check.IntervalSecs, check.TimeoutSecs, check.ExpectedStatus, check.Enabled, string(tagsJSON), time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("inserting check: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}

	check.ID = id
	check.CreatedAt = time.Now()
	check.UpdatedAt = time.Now()
	return nil
}

func (s *SQLiteStorage) GetCheck(id int64) (*Check, error) {
	row := s.db.QueryRow(`
		SELECT id, name, url, interval_seconds, timeout_seconds, expected_status, enabled, tags, created_at, updated_at
		FROM checks WHERE id = ?
	`, id)

	return s.scanCheck(row)
}

func (s *SQLiteStorage) GetCheckByURL(url string) (*Check, error) {
	row := s.db.QueryRow(`
		SELECT id, name, url, interval_seconds, timeout_seconds, expected_status, enabled, tags, created_at, updated_at
		FROM checks WHERE url = ?
	`, url)

	return s.scanCheck(row)
}

func (s *SQLiteStorage) ListChecks() ([]*Check, error) {
	rows, err := s.db.Query(`
		SELECT id, name, url, interval_seconds, timeout_seconds, expected_status, enabled, tags, created_at, updated_at
		FROM checks ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("querying checks: %w", err)
	}
	defer rows.Close()

	return s.scanChecks(rows)
}

func (s *SQLiteStorage) ListEnabledChecks() ([]*Check, error) {
	rows, err := s.db.Query(`
		SELECT id, name, url, interval_seconds, timeout_seconds, expected_status, enabled, tags, created_at, updated_at
		FROM checks WHERE enabled = 1 ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("querying enabled checks: %w", err)
	}
	defer rows.Close()

	return s.scanChecks(rows)
}

func (s *SQLiteStorage) ListChecksByTag(tag string) ([]*Check, error) {
	// Get all enabled checks and filter by tag in Go (SQLite JSON support is limited)
	checks, err := s.ListEnabledChecks()
	if err != nil {
		return nil, err
	}

	var result []*Check
	for _, check := range checks {
		for _, t := range check.Tags {
			if t == tag {
				result = append(result, check)
				break
			}
		}
	}
	return result, nil
}

func (s *SQLiteStorage) UpdateCheck(check *Check) error {
	tagsJSON, err := json.Marshal(check.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE checks SET name = ?, url = ?, interval_seconds = ?, timeout_seconds = ?, expected_status = ?, enabled = ?, tags = ?, updated_at = ?
		WHERE id = ?
	`, check.Name, check.URL, check.IntervalSecs, check.TimeoutSecs, check.ExpectedStatus, check.Enabled, string(tagsJSON), time.Now(), check.ID)
	if err != nil {
		return fmt.Errorf("updating check: %w", err)
	}

	check.UpdatedAt = time.Now()
	return nil
}

func (s *SQLiteStorage) DeleteCheck(id int64) error {
	_, err := s.db.Exec("DELETE FROM checks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting check: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) scanCheck(row *sql.Row) (*Check, error) {
	var check Check
	var tagsJSON sql.NullString

	err := row.Scan(
		&check.ID, &check.Name, &check.URL, &check.IntervalSecs, &check.TimeoutSecs,
		&check.ExpectedStatus, &check.Enabled, &tagsJSON, &check.CreatedAt, &check.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning check: %w", err)
	}

	if tagsJSON.Valid && tagsJSON.String != "" {
		if err := json.Unmarshal([]byte(tagsJSON.String), &check.Tags); err != nil {
			check.Tags = []string{}
		}
	}

	check.Status = "pending"
	return &check, nil
}

func (s *SQLiteStorage) scanChecks(rows *sql.Rows) ([]*Check, error) {
	var checks []*Check

	for rows.Next() {
		var check Check
		var tagsJSON sql.NullString

		err := rows.Scan(
			&check.ID, &check.Name, &check.URL, &check.IntervalSecs, &check.TimeoutSecs,
			&check.ExpectedStatus, &check.Enabled, &tagsJSON, &check.CreatedAt, &check.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning check: %w", err)
		}

		if tagsJSON.Valid && tagsJSON.String != "" {
			if err := json.Unmarshal([]byte(tagsJSON.String), &check.Tags); err != nil {
				check.Tags = []string{}
			}
		}

		check.Status = "pending"
		checks = append(checks, &check)
	}

	return checks, nil
}

// Check Results

func (s *SQLiteStorage) SaveResult(result *CheckResult) error {
	res, err := s.db.Exec(`
		INSERT INTO check_results (check_id, status, status_code, response_time_ms, error_message, checked_at, ssl_expires_at, ssl_days_left, ssl_issuer)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, result.CheckID, result.Status, result.StatusCode, result.ResponseTimeMs, result.ErrorMessage, time.Now(),
		result.SSLExpiresAt, result.SSLDaysLeft, result.SSLIssuer)
	if err != nil {
		return fmt.Errorf("inserting result: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}

	result.ID = id
	result.CheckedAt = time.Now()
	return nil
}

func (s *SQLiteStorage) GetResults(checkID int64, limit int, offset int) ([]*CheckResult, error) {
	rows, err := s.db.Query(`
		SELECT id, check_id, status, status_code, response_time_ms, error_message, checked_at
		FROM check_results WHERE check_id = ? ORDER BY checked_at DESC LIMIT ? OFFSET ?
	`, checkID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying results: %w", err)
	}
	defer rows.Close()

	return s.scanResults(rows)
}

func (s *SQLiteStorage) GetLatestResult(checkID int64) (*CheckResult, error) {
	row := s.db.QueryRow(`
		SELECT id, check_id, status, status_code, response_time_ms, error_message, checked_at
		FROM check_results WHERE check_id = ? ORDER BY checked_at DESC LIMIT 1
	`, checkID)

	var result CheckResult
	var errMsg sql.NullString

	err := row.Scan(
		&result.ID, &result.CheckID, &result.Status, &result.StatusCode,
		&result.ResponseTimeMs, &errMsg, &result.CheckedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning result: %w", err)
	}

	if errMsg.Valid {
		result.ErrorMessage = errMsg.String
	}

	return &result, nil
}

func (s *SQLiteStorage) GetResultsInRange(checkID int64, start, end time.Time) ([]*CheckResult, error) {
	rows, err := s.db.Query(`
		SELECT id, check_id, status, status_code, response_time_ms, error_message, checked_at
		FROM check_results WHERE check_id = ? AND checked_at BETWEEN ? AND ? ORDER BY checked_at
	`, checkID, start, end)
	if err != nil {
		return nil, fmt.Errorf("querying results: %w", err)
	}
	defer rows.Close()

	return s.scanResults(rows)
}

func (s *SQLiteStorage) GetRecentResults(checkID int64, count int) ([]*CheckResult, error) {
	return s.GetResults(checkID, count, 0)
}

func (s *SQLiteStorage) GetStats(checkID int64) (*CheckStats, error) {
	stats := &CheckStats{}

	now := time.Now()

	// 24h stats
	row := s.db.QueryRow(`
		SELECT 
			COALESCE(100.0 * SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) / NULLIF(COUNT(*), 0), 100) as uptime,
			COALESCE(AVG(CASE WHEN status = 'up' THEN response_time_ms END), 0) as avg_response
		FROM check_results 
		WHERE check_id = ? AND checked_at > ?
	`, checkID, now.Add(-24*time.Hour))

	if err := row.Scan(&stats.UptimePercent24h, &stats.AvgResponseMs24h); err != nil {
		return nil, fmt.Errorf("querying 24h stats: %w", err)
	}

	// 7d stats
	row = s.db.QueryRow(`
		SELECT 
			COALESCE(100.0 * SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) / NULLIF(COUNT(*), 0), 100) as uptime,
			COALESCE(AVG(CASE WHEN status = 'up' THEN response_time_ms END), 0) as avg_response
		FROM check_results 
		WHERE check_id = ? AND checked_at > ?
	`, checkID, now.Add(-7*24*time.Hour))

	if err := row.Scan(&stats.UptimePercent7d, &stats.AvgResponseMs7d); err != nil {
		return nil, fmt.Errorf("querying 7d stats: %w", err)
	}

	// 30d stats
	row = s.db.QueryRow(`
		SELECT 
			COALESCE(100.0 * SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) / NULLIF(COUNT(*), 0), 100) as uptime,
			COALESCE(AVG(CASE WHEN status = 'up' THEN response_time_ms END), 0) as avg_response
		FROM check_results 
		WHERE check_id = ? AND checked_at > ?
	`, checkID, now.Add(-30*24*time.Hour))

	if err := row.Scan(&stats.UptimePercent30d, &stats.AvgResponseMs30d); err != nil {
		return nil, fmt.Errorf("querying 30d stats: %w", err)
	}

	return stats, nil
}

func (s *SQLiteStorage) scanResults(rows *sql.Rows) ([]*CheckResult, error) {
	var results []*CheckResult

	for rows.Next() {
		var result CheckResult
		var errMsg sql.NullString

		err := rows.Scan(
			&result.ID, &result.CheckID, &result.Status, &result.StatusCode,
			&result.ResponseTimeMs, &errMsg, &result.CheckedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}

		if errMsg.Valid {
			result.ErrorMessage = errMsg.String
		}

		results = append(results, &result)
	}

	return results, nil
}

// Incidents

func (s *SQLiteStorage) CreateIncident(incident *Incident) error {
	res, err := s.db.Exec(`
		INSERT INTO incidents (check_id, started_at, cause)
		VALUES (?, ?, ?)
	`, incident.CheckID, incident.StartedAt, incident.Cause)
	if err != nil {
		return fmt.Errorf("inserting incident: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}

	incident.ID = id
	return nil
}

func (s *SQLiteStorage) GetIncident(id int64) (*Incident, error) {
	row := s.db.QueryRow(`
		SELECT i.id, i.check_id, i.started_at, i.ended_at, i.duration_seconds, i.cause, c.name
		FROM incidents i
		JOIN checks c ON c.id = i.check_id
		WHERE i.id = ?
	`, id)

	return s.scanIncident(row)
}

func (s *SQLiteStorage) GetActiveIncident(checkID int64) (*Incident, error) {
	row := s.db.QueryRow(`
		SELECT i.id, i.check_id, i.started_at, i.ended_at, i.duration_seconds, i.cause, c.name
		FROM incidents i
		JOIN checks c ON c.id = i.check_id
		WHERE i.check_id = ? AND i.ended_at IS NULL
	`, checkID)

	return s.scanIncident(row)
}

func (s *SQLiteStorage) CloseIncident(id int64, endedAt time.Time) error {
	// First get the incident to calculate duration
	incident, err := s.GetIncident(id)
	if err != nil {
		return err
	}
	if incident == nil {
		return fmt.Errorf("incident not found")
	}

	duration := int(endedAt.Sub(incident.StartedAt).Seconds())

	_, err = s.db.Exec(`
		UPDATE incidents SET ended_at = ?, duration_seconds = ? WHERE id = ?
	`, endedAt, duration, id)
	if err != nil {
		return fmt.Errorf("closing incident: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) ListIncidents(limit int, offset int) ([]*Incident, error) {
	rows, err := s.db.Query(`
		SELECT i.id, i.check_id, i.started_at, i.ended_at, i.duration_seconds, i.cause, c.name
		FROM incidents i
		JOIN checks c ON c.id = i.check_id
		ORDER BY i.started_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying incidents: %w", err)
	}
	defer rows.Close()

	return s.scanIncidents(rows)
}

func (s *SQLiteStorage) ListIncidentsForCheck(checkID int64, limit int) ([]*Incident, error) {
	rows, err := s.db.Query(`
		SELECT i.id, i.check_id, i.started_at, i.ended_at, i.duration_seconds, i.cause, c.name
		FROM incidents i
		JOIN checks c ON c.id = i.check_id
		WHERE i.check_id = ?
		ORDER BY i.started_at DESC LIMIT ?
	`, checkID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying incidents: %w", err)
	}
	defer rows.Close()

	return s.scanIncidents(rows)
}

func (s *SQLiteStorage) scanIncident(row *sql.Row) (*Incident, error) {
	var incident Incident
	var endedAt sql.NullTime
	var duration sql.NullInt64
	var cause sql.NullString

	err := row.Scan(
		&incident.ID, &incident.CheckID, &incident.StartedAt, &endedAt,
		&duration, &cause, &incident.CheckName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning incident: %w", err)
	}

	if endedAt.Valid {
		incident.EndedAt = &endedAt.Time
	}
	if duration.Valid {
		incident.DurationSeconds = int(duration.Int64)
	}
	if cause.Valid {
		incident.Cause = cause.String
	}

	return &incident, nil
}

func (s *SQLiteStorage) scanIncidents(rows *sql.Rows) ([]*Incident, error) {
	var incidents []*Incident

	for rows.Next() {
		var incident Incident
		var endedAt sql.NullTime
		var duration sql.NullInt64
		var cause sql.NullString

		err := rows.Scan(
			&incident.ID, &incident.CheckID, &incident.StartedAt, &endedAt,
			&duration, &cause, &incident.CheckName,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning incident: %w", err)
		}

		if endedAt.Valid {
			incident.EndedAt = &endedAt.Time
		}
		if duration.Valid {
			incident.DurationSeconds = int(duration.Int64)
		}
		if cause.Valid {
			incident.Cause = cause.String
		}

		incidents = append(incidents, &incident)
	}

	return incidents, nil
}

// Alert Log

func (s *SQLiteStorage) LogAlert(log *AlertLog) error {
	res, err := s.db.Exec(`
		INSERT INTO alert_log (incident_id, channel, sent_at, success, error_message)
		VALUES (?, ?, ?, ?, ?)
	`, log.IncidentID, log.Channel, time.Now(), log.Success, log.ErrorMessage)
	if err != nil {
		return fmt.Errorf("inserting alert log: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}

	log.ID = id
	log.SentAt = time.Now()
	return nil
}

func (s *SQLiteStorage) GetLastAlertForIncident(incidentID int64, channel string) (*AlertLog, error) {
	row := s.db.QueryRow(`
		SELECT id, incident_id, channel, sent_at, success, error_message
		FROM alert_log WHERE incident_id = ? AND channel = ? ORDER BY sent_at DESC LIMIT 1
	`, incidentID, channel)

	var log AlertLog
	var errMsg sql.NullString

	err := row.Scan(&log.ID, &log.IncidentID, &log.Channel, &log.SentAt, &log.Success, &errMsg)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning alert log: %w", err)
	}

	if errMsg.Valid {
		log.ErrorMessage = errMsg.String
	}

	return &log, nil
}

// Aggregates

func (s *SQLiteStorage) CreateHourlyAggregate(agg *HourlyAggregate) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO hourly_aggregates 
		(check_id, hour, total_checks, success_count, failure_count, avg_response_ms, min_response_ms, max_response_ms, uptime_percent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, agg.CheckID, agg.Hour, agg.TotalChecks, agg.SuccessCount, agg.FailureCount,
		agg.AvgResponseMs, agg.MinResponseMs, agg.MaxResponseMs, agg.UptimePercent)
	if err != nil {
		return fmt.Errorf("creating hourly aggregate: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) GetHourlyAggregates(checkID int64, start, end time.Time) ([]*HourlyAggregate, error) {
	rows, err := s.db.Query(`
		SELECT id, check_id, hour, total_checks, success_count, failure_count, 
		       avg_response_ms, min_response_ms, max_response_ms, uptime_percent
		FROM hourly_aggregates 
		WHERE check_id = ? AND hour BETWEEN ? AND ?
		ORDER BY hour
	`, checkID, start, end)
	if err != nil {
		return nil, fmt.Errorf("querying hourly aggregates: %w", err)
	}
	defer rows.Close()

	var aggregates []*HourlyAggregate
	for rows.Next() {
		var agg HourlyAggregate
		err := rows.Scan(&agg.ID, &agg.CheckID, &agg.Hour, &agg.TotalChecks,
			&agg.SuccessCount, &agg.FailureCount, &agg.AvgResponseMs,
			&agg.MinResponseMs, &agg.MaxResponseMs, &agg.UptimePercent)
		if err != nil {
			return nil, fmt.Errorf("scanning hourly aggregate: %w", err)
		}
		aggregates = append(aggregates, &agg)
	}
	return aggregates, nil
}

// Maintenance

func (s *SQLiteStorage) AggregateResults(olderThan time.Time) error {
	// Get all checks
	checks, err := s.ListChecks()
	if err != nil {
		return fmt.Errorf("listing checks for aggregation: %w", err)
	}

	for _, check := range checks {
		// Find all hours with results older than the cutoff
		rows, err := s.db.Query(`
			SELECT 
				strftime('%Y-%m-%d %H:00:00', checked_at) as hour,
				COUNT(*) as total,
				SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END) as failure,
				AVG(CASE WHEN status = 'up' THEN response_time_ms END) as avg_ms,
				MIN(CASE WHEN status = 'up' THEN response_time_ms END) as min_ms,
				MAX(CASE WHEN status = 'up' THEN response_time_ms END) as max_ms
			FROM check_results
			WHERE check_id = ? AND checked_at < ?
			GROUP BY strftime('%Y-%m-%d %H:00:00', checked_at)
		`, check.ID, olderThan)
		if err != nil {
			continue
		}

		for rows.Next() {
			var hourStr string
			var total, success, failure int
			var avgMs, minMs, maxMs *int

			if err := rows.Scan(&hourStr, &total, &success, &failure, &avgMs, &minMs, &maxMs); err != nil {
				continue
			}

			hour, _ := time.Parse("2006-01-02 15:04:05", hourStr)
			
			agg := &HourlyAggregate{
				CheckID:       check.ID,
				Hour:          hour,
				TotalChecks:   total,
				SuccessCount:  success,
				FailureCount:  failure,
				UptimePercent: float64(success) / float64(total) * 100,
			}
			if avgMs != nil {
				agg.AvgResponseMs = *avgMs
			}
			if minMs != nil {
				agg.MinResponseMs = *minMs
			}
			if maxMs != nil {
				agg.MaxResponseMs = *maxMs
			}

			s.CreateHourlyAggregate(agg)
		}
		rows.Close()
	}

	return nil
}

func (s *SQLiteStorage) CleanupOldResults(olderThan time.Time) error {
	_, err := s.db.Exec("DELETE FROM check_results WHERE checked_at < ?", olderThan)
	if err != nil {
		return fmt.Errorf("cleaning up old results: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) CleanupOldAggregates(olderThan time.Time) error {
	_, err := s.db.Exec("DELETE FROM hourly_aggregates WHERE hour < ?", olderThan)
	if err != nil {
		return fmt.Errorf("cleaning up old aggregates: %w", err)
	}
	return nil
}
