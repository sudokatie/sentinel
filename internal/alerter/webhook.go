package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/katieblackabee/sentinel/internal/config"
)

// SlackSender sends alerts to Slack via webhook
type SlackSender struct {
	config *config.SlackConfig
	client *http.Client
}

// DiscordSender sends alerts to Discord via webhook
type DiscordSender struct {
	config *config.DiscordConfig
	client *http.Client
}

// SlackMessage is the Slack webhook payload
type SlackMessage struct {
	Text        string            `json:"text,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

type SlackAttachment struct {
	Color  string `json:"color"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Footer string `json:"footer"`
	Ts     int64  `json:"ts"`
}

// DiscordMessage is the Discord webhook payload
type DiscordMessage struct {
	Content string         `json:"content,omitempty"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Color       int           `json:"color"`
	Footer      DiscordFooter `json:"footer"`
	Timestamp   string        `json:"timestamp"`
}

type DiscordFooter struct {
	Text string `json:"text"`
}

func NewSlackSender(cfg *config.SlackConfig) *SlackSender {
	return &SlackSender{
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func NewDiscordSender(cfg *config.DiscordConfig) *DiscordSender {
	return &DiscordSender{
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackSender) Send(alert *Alert) error {
	msg := s.buildMessage(alert)

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling slack message: %w", err)
	}

	req, err := http.NewRequest("POST", s.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending slack webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *SlackSender) buildMessage(alert *Alert) *SlackMessage {
	var color, title, text string

	switch alert.Type {
	case "down":
		color = "danger" // red
		title = fmt.Sprintf("🔴 DOWN: %s", alert.Check.Name)
		text = fmt.Sprintf("*URL:* %s\n*Error:* %s", alert.Check.URL, alert.Error)
	case "recovery":
		color = "good" // green
		title = fmt.Sprintf("✅ RECOVERED: %s", alert.Check.Name)
		duration := "unknown"
		if alert.Incident != nil {
			duration = alert.Incident.DurationString()
		}
		text = fmt.Sprintf("*URL:* %s\n*Downtime:* %s", alert.Check.URL, duration)
	case "ssl_expiry":
		color = "warning" // yellow
		title = fmt.Sprintf("⚠️ SSL EXPIRING: %s", alert.Check.Name)
		text = fmt.Sprintf("*URL:* %s\n*Warning:* %s", alert.Check.URL, alert.Error)
	default:
		color = "danger"
		title = fmt.Sprintf("Alert: %s", alert.Check.Name)
		text = alert.Error
	}

	return &SlackMessage{
		Attachments: []SlackAttachment{
			{
				Color:  color,
				Title:  title,
				Text:   text,
				Footer: "Sentinel Uptime Monitor",
				Ts:     alert.Timestamp.Unix(),
			},
		},
	}
}

func (d *DiscordSender) Send(alert *Alert) error {
	msg := d.buildMessage(alert)

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling discord message: %w", err)
	}

	req, err := http.NewRequest("POST", d.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending discord webhook: %w", err)
	}
	defer resp.Body.Close()

	// Discord returns 204 No Content on success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (d *DiscordSender) buildMessage(alert *Alert) *DiscordMessage {
	var color int
	var title, description string

	switch alert.Type {
	case "down":
		color = 15158332 // red (#E74C3C)
		title = fmt.Sprintf("🔴 DOWN: %s", alert.Check.Name)
		description = fmt.Sprintf("**URL:** %s\n**Error:** %s", alert.Check.URL, alert.Error)
	case "recovery":
		color = 3066993 // green (#2ECC71)
		title = fmt.Sprintf("✅ RECOVERED: %s", alert.Check.Name)
		duration := "unknown"
		if alert.Incident != nil {
			duration = alert.Incident.DurationString()
		}
		description = fmt.Sprintf("**URL:** %s\n**Downtime:** %s", alert.Check.URL, duration)
	case "ssl_expiry":
		color = 16776960 // yellow (#FFFF00)
		title = fmt.Sprintf("⚠️ SSL EXPIRING: %s", alert.Check.Name)
		description = fmt.Sprintf("**URL:** %s\n**Warning:** %s", alert.Check.URL, alert.Error)
	default:
		color = 15158332
		title = fmt.Sprintf("Alert: %s", alert.Check.Name)
		description = alert.Error
	}

	return &DiscordMessage{
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: description,
				Color:       color,
				Footer:      DiscordFooter{Text: "Sentinel Uptime Monitor"},
				Timestamp:   alert.Timestamp.Format(time.RFC3339),
			},
		},
	}
}
