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

	// mu защищает targets: проверки идут в goroutine, а /status может читать их параллельно.
	mu      sync.RWMutex
	targets map[string]TargetState
}

// TargetState - последнее известное состояние одной цели.
// Возможные state:
// pending - проверка еще не успела выполниться;
// up - последняя проверка успешна;
// suspect - есть ошибка, но порог consecutive_failures еще не достигнут;
// down - ошибок подряд достаточно, чтобы считать цель недоступной.
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

// RenewalState - вычисленное состояние оплаты на текущую дату.
// Конфиг хранит due_date, а days_left/state считаются на лету.
type RenewalState struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	DueDate  string `json:"due_date"`
	DaysLeft int    `json:"days_left"`
	State    string `json:"state"`
}

// Snapshot - полный срез состояния, который отдает /status и будет использовать Telegram.
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

// Start запускает бесконечные циклы проверок.
// На каждую цель создается отдельная goroutine, поэтому медленная цель не тормозит остальные.
func (m *Monitor) Start(ctx context.Context) {
	for _, target := range m.cfg.Targets {
		target := target
		go m.runTargetLoop(ctx, target)
	}
}

// CheckAll нужен для режима -once: выполнить все проверки прямо сейчас и вернуть управление.
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

// Snapshot копирует состояние под read-lock, чтобы HTTP handler не видел полузаписанные данные.
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

// runTargetLoop сразу выполняет первую проверку, затем повторяет ее по interval из конфига.
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

// checkTarget запускает конкретный checker и обновляет state machine для цели.
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
		// Любая успешная проверка полностью сбрасывает счетчик ошибок.
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

// renewalStates переводит due_date из конфига в удобный статус для человека.
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
