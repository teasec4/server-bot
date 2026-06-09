package checks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"server-bot/internal/config"
)

type Result struct {
	TargetID    string    `json:"target_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	OK          bool      `json:"ok"`
	CheckedAt   time.Time `json:"checked_at"`
	DurationMS  int64     `json:"duration_ms"`
	HTTPStatus  int       `json:"http_status,omitempty"`
	Error       string    `json:"error,omitempty"`
	Description string    `json:"description"`
}

func RunHTTP(ctx context.Context, target config.TargetConfig) Result {
	startedAt := time.Now()
	result := Result{
		TargetID:  target.ID,
		Name:      target.Name,
		Type:      target.Type,
		CheckedAt: startedAt,
	}

	requestCtx, cancel := context.WithTimeout(ctx, target.Timeout.Duration)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, target.Method, target.URL, nil)
	if err != nil {
		return finish(result, startedAt, fmt.Errorf("build request: %w", err), "")
	}
	request.Header.Set("User-Agent", "server-bot/0.1")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return finish(result, startedAt, err, "")
	}
	defer response.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	result.HTTPStatus = response.StatusCode
	if response.StatusCode != target.ExpectedStatus {
		return finish(result, startedAt, nil, fmt.Sprintf("unexpected HTTP status %d, expected %d", response.StatusCode, target.ExpectedStatus))
	}

	return finish(result, startedAt, nil, "ok")
}

func finish(result Result, startedAt time.Time, err error, description string) Result {
	result.DurationMS = time.Since(startedAt).Milliseconds()
	if err != nil {
		result.OK = false
		result.Error = err.Error()
		result.Description = "request failed"
		return result
	}

	result.OK = description == "ok"
	result.Description = description
	return result
}
