package alerter

import (
	"fmt"
	"time"

	"github.com/katieblackabee/sentinel/internal/config"
	"github.com/katieblackabee/sentinel/internal/storage"
)

type Manager struct {
	config  *config.AlertsConfig
	storage storage.Storage
	email   *EmailSender
	slack   *SlackSender
	discord *DiscordSender
}

type Alert struct {
	Type      string // "down" or "recovery"
	Check     *storage.Check
	Incident  *storage.Incident
	Error     string
	Timestamp time.Time
}

func NewManager(cfg *config.AlertsConfig, store storage.Storage) *Manager {
	m := &Manager{
		config:  cfg,
		storage: store,
	}

	if cfg.Email.Enabled {
		m.email = NewEmailSender(&cfg.Email)
	}

	if cfg.Slack.Enabled {
		m.slack = NewSlackSender(&cfg.Slack)
	}

	if cfg.Discord.Enabled {
		m.discord = NewDiscordSender(&cfg.Discord)
	}

	return m
}

func (m *Manager) SendDownAlert(check *storage.Check, incident *storage.Incident, errorMsg string) error {
	alert := &Alert{
		Type:      "down",
		Check:     check,
		Incident:  incident,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}

	return m.sendAlert(alert)
}

func (m *Manager) SendRecoveryAlert(check *storage.Check, incident *storage.Incident) error {
	if !m.config.RecoveryNotification {
		return nil
	}

	alert := &Alert{
		Type:      "recovery",
		Check:     check,
		Incident:  incident,
		Timestamp: time.Now(),
	}

	return m.sendAlert(alert)
}

func (m *Manager) SendSSLExpiryAlert(check *storage.Check, daysLeft int, expiresAt time.Time) error {
	if m.config.SSLExpiryDays == 0 {
		return nil // SSL alerts disabled
	}

	alert := &Alert{
		Type:      "ssl_expiry",
		Check:     check,
		Error:     fmt.Sprintf("SSL certificate expires in %d days (on %s)", daysLeft, expiresAt.Format("Jan 2, 2006")),
		Timestamp: time.Now(),
	}

	return m.sendAlert(alert)
}

func (m *Manager) sendAlert(alert *Alert) error {
	// Check cooldown
	if !m.shouldSendAlert(alert) {
		return nil
	}

	var lastErr error

	// Send via email if enabled
	if m.email != nil {
		if err := m.email.Send(alert); err != nil {
			lastErr = err
			m.logAlert(alert, "email", false, err.Error())
		} else {
			m.logAlert(alert, "email", true, "")
		}
	}

	// Send via Slack if enabled
	if m.slack != nil {
		if err := m.slack.Send(alert); err != nil {
			lastErr = err
			m.logAlert(alert, "slack", false, err.Error())
		} else {
			m.logAlert(alert, "slack", true, "")
		}
	}

	// Send via Discord if enabled
	if m.discord != nil {
		if err := m.discord.Send(alert); err != nil {
			lastErr = err
			m.logAlert(alert, "discord", false, err.Error())
		} else {
			m.logAlert(alert, "discord", true, "")
		}
	}

	return lastErr
}

func (m *Manager) shouldSendAlert(alert *Alert) bool {
	if alert.Incident == nil {
		return true
	}

	// Check cooldown period
	cooldown := time.Duration(m.config.CooldownMinutes) * time.Minute

	// Get last alert for this incident
	lastAlert, err := m.storage.GetLastAlertForIncident(alert.Incident.ID, "email")
	if err != nil {
		return true // On error, allow the alert
	}

	if lastAlert != nil && lastAlert.Success {
		if time.Since(lastAlert.SentAt) < cooldown {
			return false
		}
	}

	return true
}

func (m *Manager) logAlert(alert *Alert, channel string, success bool, errMsg string) {
	if alert.Incident == nil {
		return
	}

	log := &storage.AlertLog{
		IncidentID:   alert.Incident.ID,
		Channel:      channel,
		Success:      success,
		ErrorMessage: errMsg,
	}

	if err := m.storage.LogAlert(log); err != nil {
		fmt.Printf("failed to log alert: %v\n", err)
	}
}
