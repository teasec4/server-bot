package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("duration must be a string like \"30s\": %w", err)
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value, err)
	}
	if parsed <= 0 {
		return errors.New("duration must be positive")
	}

	d.Duration = parsed
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

type Config struct {
	Listen   string          `json:"listen"`
	Targets  []TargetConfig  `json:"targets"`
	Renewals []RenewalConfig `json:"renewals"`
}

type TargetConfig struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	URL                 string   `json:"url"`
	Method              string   `json:"method"`
	Interval            Duration `json:"interval"`
	Timeout             Duration `json:"timeout"`
	ExpectedStatus      int      `json:"expected_status"`
	ConsecutiveFailures int      `json:"consecutive_failures"`
}

type RenewalConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	DueDate  string `json:"due_date"`
	WarnDays []int  `json:"warn_days"`
}

func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := cfg.ApplyDefaultsAndValidate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (cfg *Config) ApplyDefaultsAndValidate() error {
	if strings.TrimSpace(cfg.Listen) == "" {
		cfg.Listen = ":8080"
	}

	seenTargets := make(map[string]struct{}, len(cfg.Targets))
	for i := range cfg.Targets {
		target := &cfg.Targets[i]
		target.ID = strings.TrimSpace(target.ID)
		target.Name = strings.TrimSpace(target.Name)
		target.Type = strings.ToLower(strings.TrimSpace(target.Type))
		target.Method = strings.ToUpper(strings.TrimSpace(target.Method))
		target.URL = strings.TrimSpace(target.URL)

		if target.ID == "" {
			return fmt.Errorf("targets[%d].id is required", i)
		}
		if _, ok := seenTargets[target.ID]; ok {
			return fmt.Errorf("duplicate target id %q", target.ID)
		}
		seenTargets[target.ID] = struct{}{}

		if target.Name == "" {
			target.Name = target.ID
		}
		if target.Type == "" {
			target.Type = "http"
		}
		if target.Type != "http" {
			return fmt.Errorf("target %q has unsupported type %q", target.ID, target.Type)
		}
		if target.Method == "" {
			target.Method = "GET"
		}
		if target.Method != "GET" && target.Method != "HEAD" {
			return fmt.Errorf("target %q has unsupported method %q", target.ID, target.Method)
		}
		if target.URL == "" {
			return fmt.Errorf("target %q url is required", target.ID)
		}
		parsedURL, err := url.ParseRequestURI(target.URL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("target %q has invalid url %q", target.ID, target.URL)
		}
		if target.Interval.Duration == 0 {
			target.Interval.Duration = time.Minute
		}
		if target.Timeout.Duration == 0 {
			target.Timeout.Duration = 5 * time.Second
		}
		if target.ExpectedStatus == 0 {
			target.ExpectedStatus = 200
		}
		if target.ConsecutiveFailures == 0 {
			target.ConsecutiveFailures = 2
		}
		if target.ConsecutiveFailures < 1 {
			return fmt.Errorf("target %q consecutive_failures must be at least 1", target.ID)
		}
	}

	seenRenewals := make(map[string]struct{}, len(cfg.Renewals))
	for i := range cfg.Renewals {
		renewal := &cfg.Renewals[i]
		renewal.ID = strings.TrimSpace(renewal.ID)
		renewal.Name = strings.TrimSpace(renewal.Name)

		if renewal.ID == "" {
			return fmt.Errorf("renewals[%d].id is required", i)
		}
		if _, ok := seenRenewals[renewal.ID]; ok {
			return fmt.Errorf("duplicate renewal id %q", renewal.ID)
		}
		seenRenewals[renewal.ID] = struct{}{}
		if renewal.Name == "" {
			renewal.Name = renewal.ID
		}
		if _, err := time.Parse("2006-01-02", renewal.DueDate); err != nil {
			return fmt.Errorf("renewal %q due_date must use YYYY-MM-DD: %w", renewal.ID, err)
		}
		if len(renewal.WarnDays) == 0 {
			renewal.WarnDays = []int{14, 7, 3, 1}
		}
		for _, day := range renewal.WarnDays {
			if day < 0 {
				return fmt.Errorf("renewal %q warn_days cannot contain negative values", renewal.ID)
			}
		}
	}

	return nil
}
