package monitor

import (
	"context"
	"sync"
	"time"

	"server-bot/internal/checks"
	"server-bot/internal/config"
)

type Monitor struct {
	cfg *config.Config

	mu      sync.RWMutex
	targets map[string]TargetState
}

type TargetState struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Type                string         `json:"type"`
	URL                 string         `json:"url"`
	State               string         `json:"state"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
	FailureThreshold    int            `json:"failure_threshold"`
	LastResult          *checks.Result `json:"last_result,omitempty"`
	NextCheckAt         time.Time      `json:"next_check_at,omitempty"`
}

type RenewalState struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	DueDate  string `json:"due_date"`
	DaysLeft int    `json:"days_left"`
	State    string `json:"state"`
}

type Snapshot struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Targets     []TargetState  `json:"targets"`
	Renewals    []RenewalState `json:"renewals"`
}

func New(cfg *config.Config) *Monitor {
	targets := make(map[string]TargetState, len(cfg.Targets))
	for _, target := range cfg.Targets {
		targets[target.ID] = TargetState{
			ID:               target.ID,
			Name:             target.Name,
			Type:             target.Type,
			URL:              target.URL,
			State:            "pending",
			FailureThreshold: target.ConsecutiveFailures,
		}
	}

	return &Monitor{
		cfg:     cfg,
		targets: targets,
	}
}

func (m *Monitor) Start(ctx context.Context) {
	for _, target := range m.cfg.Targets {
		target := target
		go m.runTargetLoop(ctx, target)
	}
}

func (m *Monitor) CheckAll(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(m.cfg.Targets))
	for _, target := range m.cfg.Targets {
		target := target
		go func() {
			defer wg.Done()
			m.checkTarget(ctx, target)
		}()
	}
	wg.Wait()
}

func (m *Monitor) Snapshot(now time.Time) Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]TargetState, 0, len(m.cfg.Targets))
	for _, cfgTarget := range m.cfg.Targets {
		targets = append(targets, m.targets[cfgTarget.ID])
	}

	return Snapshot{
		GeneratedAt: now,
		Targets:     targets,
		Renewals:    renewalStates(m.cfg.Renewals, now),
	}
}

func (m *Monitor) runTargetLoop(ctx context.Context, target config.TargetConfig) {
	m.checkTarget(ctx, target)

	ticker := time.NewTicker(target.Interval.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkTarget(ctx, target)
		}
	}
}

func (m *Monitor) checkTarget(ctx context.Context, target config.TargetConfig) {
	var result checks.Result
	switch target.Type {
	case "http":
		result = checks.RunHTTP(ctx, target)
	default:
		result = checks.Result{
			TargetID:    target.ID,
			Name:        target.Name,
			Type:        target.Type,
			CheckedAt:   time.Now(),
			Error:       "unsupported target type",
			Description: "unsupported target type",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.targets[target.ID]
	state.LastResult = &result
	state.NextCheckAt = time.Now().Add(target.Interval.Duration)
	if result.OK {
		state.State = "up"
		state.ConsecutiveFailures = 0
	} else {
		state.ConsecutiveFailures++
		if state.ConsecutiveFailures >= target.ConsecutiveFailures {
			state.State = "down"
		} else {
			state.State = "suspect"
		}
	}
	m.targets[target.ID] = state
}

func renewalStates(renewals []config.RenewalConfig, now time.Time) []RenewalState {
	states := make([]RenewalState, 0, len(renewals))
	location := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	for _, renewal := range renewals {
		dueDate, err := time.ParseInLocation("2006-01-02", renewal.DueDate, location)
		if err != nil {
			continue
		}

		daysLeft := int(dueDate.Sub(today).Hours() / 24)
		state := "ok"
		if daysLeft < 0 {
			state = "expired"
		} else {
			for _, warnDay := range renewal.WarnDays {
				if daysLeft <= warnDay {
					state = "warning"
					break
				}
			}
		}

		states = append(states, RenewalState{
			ID:       renewal.ID,
			Name:     renewal.Name,
			DueDate:  renewal.DueDate,
			DaysLeft: daysLeft,
			State:    state,
		})
	}

	return states
}
