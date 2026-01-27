package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Alerts    AlertsConfig    `yaml:"alerts"`
	Retention RetentionConfig `yaml:"retention"`
	Checks    []CheckConfig   `yaml:"checks"`
}

type ServerConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type AlertsConfig struct {
	ConsecutiveFailures  int         `yaml:"consecutive_failures"`
	RecoveryNotification bool        `yaml:"recovery_notification"`
	CooldownMinutes      int         `yaml:"cooldown_minutes"`
	Email                EmailConfig `yaml:"email"`
}

type EmailConfig struct {
	Enabled      bool     `yaml:"enabled"`
	SMTPHost     string   `yaml:"smtp_host"`
	SMTPPort     int      `yaml:"smtp_port"`
	SMTPUser     string   `yaml:"smtp_user"`
	SMTPPassword string   `yaml:"smtp_password"`
	SMTPTLS      bool     `yaml:"smtp_tls"`
	FromAddress  string   `yaml:"from_address"`
	ToAddresses  []string `yaml:"to_addresses"`
}

type RetentionConfig struct {
	ResultsDays    int `yaml:"results_days"`
	AggregatesDays int `yaml:"aggregates_days"`
}

type CheckConfig struct {
	Name           string   `yaml:"name"`
	URL            string   `yaml:"url"`
	Interval       string   `yaml:"interval"`
	Timeout        string   `yaml:"timeout"`
	ExpectedStatus int      `yaml:"expected_status"`
	Enabled        *bool    `yaml:"enabled"`
	Tags           []string `yaml:"tags"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 3000,
		},
		Database: DatabaseConfig{
			Path: "./sentinel.db",
		},
		Alerts: AlertsConfig{
			ConsecutiveFailures:  2,
			RecoveryNotification: true,
			CooldownMinutes:      5,
			Email: EmailConfig{
				Enabled:  false,
				SMTPPort: 587,
				SMTPTLS:  true,
			},
		},
		Retention: RetentionConfig{
			ResultsDays:    7,
			AggregatesDays: 90,
		},
		Checks: []CheckConfig{},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return config, nil
}

func LoadWithEnv(path string) (*Config, error) {
	config, err := Load(path)
	if err != nil {
		return nil, err
	}

	applyEnvOverrides(config)

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return config, nil
}

func applyEnvOverrides(c *Config) {
	if v := os.Getenv("SENTINEL_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("SENTINEL_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("SENTINEL_BASE_URL"); v != "" {
		c.Server.BaseURL = v
	}
	if v := os.Getenv("SENTINEL_DB_PATH"); v != "" {
		c.Database.Path = v
	}
	if v := os.Getenv("SENTINEL_SMTP_HOST"); v != "" {
		c.Alerts.Email.SMTPHost = v
	}
	if v := os.Getenv("SENTINEL_SMTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Alerts.Email.SMTPPort = port
		}
	}
	if v := os.Getenv("SENTINEL_SMTP_USER"); v != "" {
		c.Alerts.Email.SMTPUser = v
	}
	if v := os.Getenv("SENTINEL_SMTP_PASSWORD"); v != "" {
		c.Alerts.Email.SMTPPassword = v
	}
	if v := os.Getenv("SENTINEL_SMTP_FROM"); v != "" {
		c.Alerts.Email.FromAddress = v
	}
	if v := os.Getenv("SENTINEL_SMTP_TO"); v != "" {
		c.Alerts.Email.ToAddresses = strings.Split(v, ",")
	}
	if v := os.Getenv("SENTINEL_EMAIL_ENABLED"); v != "" {
		c.Alerts.Email.Enabled = v == "true" || v == "1"
	}
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}

	if c.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}

	if c.Alerts.ConsecutiveFailures < 1 {
		return fmt.Errorf("consecutive_failures must be at least 1")
	}

	if c.Alerts.CooldownMinutes < 0 {
		return fmt.Errorf("cooldown_minutes cannot be negative")
	}

	if c.Alerts.Email.Enabled {
		if c.Alerts.Email.SMTPHost == "" {
			return fmt.Errorf("smtp_host is required when email is enabled")
		}
		if c.Alerts.Email.SMTPPort < 1 || c.Alerts.Email.SMTPPort > 65535 {
			return fmt.Errorf("invalid smtp_port: %d", c.Alerts.Email.SMTPPort)
		}
		if c.Alerts.Email.FromAddress == "" {
			return fmt.Errorf("from_address is required when email is enabled")
		}
		if len(c.Alerts.Email.ToAddresses) == 0 {
			return fmt.Errorf("to_addresses is required when email is enabled")
		}
	}

	for i, check := range c.Checks {
		if check.Name == "" {
			return fmt.Errorf("check[%d]: name is required", i)
		}
		if check.URL == "" {
			return fmt.Errorf("check[%d]: url is required", i)
		}
		if check.Interval != "" {
			if _, err := time.ParseDuration(check.Interval); err != nil {
				return fmt.Errorf("check[%d]: invalid interval %q: %w", i, check.Interval, err)
			}
		}
		if check.Timeout != "" {
			if _, err := time.ParseDuration(check.Timeout); err != nil {
				return fmt.Errorf("check[%d]: invalid timeout %q: %w", i, check.Timeout, err)
			}
		}
	}

	if c.Retention.ResultsDays < 1 {
		return fmt.Errorf("results_days must be at least 1")
	}

	return nil
}

func (c *CheckConfig) GetInterval() time.Duration {
	if c.Interval == "" {
		return time.Minute
	}
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return time.Minute
	}
	return d
}

func (c *CheckConfig) GetTimeout() time.Duration {
	if c.Timeout == "" {
		return 10 * time.Second
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 10 * time.Second
	}
	return d
}

func (c *CheckConfig) GetExpectedStatus() int {
	if c.ExpectedStatus == 0 {
		return 200
	}
	return c.ExpectedStatus
}

func (c *CheckConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}
