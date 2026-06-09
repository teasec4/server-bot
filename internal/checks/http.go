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

// RunHTTP выполняет одну HTTP-проверку и возвращает самодостаточный результат.
// Функция ничего не знает про Telegram, расписание и историю - только один запрос.
func RunHTTP(ctx context.Context, target config.TargetConfig) Result {
	startedAt := time.Now()
	result := Result{
		TargetID:  target.ID,
		Name:      target.Name,
		Type:      target.Type,
		CheckedAt: startedAt,
	}

	// У каждой цели свой timeout, чтобы зависший сайт не блокировал весь мониторинг.
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

	// Читаем маленький кусок тела, чтобы HTTP-клиент мог корректно переиспользовать соединение.
	// Сам контент страницы нам пока не важен: проверяем доступность и статус-код.
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	result.HTTPStatus = response.StatusCode
	if response.StatusCode != target.ExpectedStatus {
		return finish(result, startedAt, nil, fmt.Sprintf("unexpected HTTP status %d, expected %d", response.StatusCode, target.ExpectedStatus))
	}

	return finish(result, startedAt, nil, "ok")
}

// finish заполняет общие поля результата: длительность, ok/error и короткое описание.
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
