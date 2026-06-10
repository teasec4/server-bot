package telegram

import (
	"fmt"
	"strings"
	"time"

	"server-bot/internal/monitor"
)

func FormatTargetEvent(event monitor.TargetEvent) string {
	target := event.Target
	result := target.LastResult

	var builder strings.Builder
	if event.CurrentState == "down" {
		builder.WriteString("ALERT: цель недоступна\n")
	} else {
		builder.WriteString("RECOVERY: цель восстановилась\n")
	}

	builder.WriteString("name: ")
	builder.WriteString(target.Name)
	builder.WriteString("\n")
	builder.WriteString("id: ")
	builder.WriteString(target.ID)
	builder.WriteString("\n")
	builder.WriteString("state: ")
	builder.WriteString(event.PreviousState)
	builder.WriteString(" -> ")
	builder.WriteString(event.CurrentState)
	builder.WriteString("\n")
	builder.WriteString("url: ")
	builder.WriteString(target.URL)
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("failures: %d/%d\n", target.ConsecutiveFailures, target.FailureThreshold))

	if result == nil {
		return strings.TrimSpace(builder.String())
	}

	builder.WriteString(fmt.Sprintf("duration: %dms\n", result.DurationMS))
	if result.HTTPStatus != 0 {
		builder.WriteString(fmt.Sprintf("http_status: %d\n", result.HTTPStatus))
	}
	if result.Error != "" {
		builder.WriteString("error: ")
		builder.WriteString(result.Error)
		builder.WriteString("\n")
	} else if result.Description != "" {
		builder.WriteString("description: ")
		builder.WriteString(result.Description)
		builder.WriteString("\n")
	}
	builder.WriteString("checked_at: ")
	builder.WriteString(result.CheckedAt.Format(time.RFC3339))

	return strings.TrimSpace(builder.String())
}

func FormatSnapshot(snapshot monitor.Snapshot) string {
	var builder strings.Builder
	builder.WriteString("Отчет server-bot\n")
	builder.WriteString("generated_at: ")
	builder.WriteString(snapshot.GeneratedAt.Format(time.RFC3339))
	builder.WriteString("\n\n")

	if len(snapshot.Targets) == 0 {
		builder.WriteString("targets: нет настроенных проверок\n")
	} else {
		builder.WriteString("targets:\n")
		for _, target := range snapshot.Targets {
			builder.WriteString("- ")
			builder.WriteString(target.Name)
			builder.WriteString(" [")
			builder.WriteString(target.State)
			builder.WriteString("]")
			if target.LastResult != nil {
				if target.LastResult.HTTPStatus != 0 {
					builder.WriteString(fmt.Sprintf(" http=%d", target.LastResult.HTTPStatus))
				}
				builder.WriteString(fmt.Sprintf(" %dms", target.LastResult.DurationMS))
				if target.LastResult.Error != "" {
					builder.WriteString(" error=")
					builder.WriteString(target.LastResult.Error)
				}
			}
			builder.WriteString("\n")
		}
	}

	if len(snapshot.Renewals) > 0 {
		builder.WriteString("\nrenewals:\n")
		for _, renewal := range snapshot.Renewals {
			builder.WriteString(fmt.Sprintf("- %s [%s] %s, days_left=%d\n", renewal.Name, renewal.State, renewal.DueDate, renewal.DaysLeft))
		}
	}

	return strings.TrimSpace(builder.String())
}

func helpMessage() string {
	return strings.Join([]string{
		"Команды server-bot:",
		"/status или кнопка Отчет - показать последний статус",
		"/check или кнопка Проверить сейчас - запустить проверки вручную",
		"/ping или кнопка Проверить соединение - проверить связь с ботом",
		"/whoami - показать chat_id/user_id",
	}, "\n")
}

func unpairedHelpMessage() string {
	return strings.Join([]string{
		"Бот еще не привязан к admin chat.",
		"1. Открой server logs.",
		"2. Найди telegram pairing code.",
		"3. Отправь сюда: /pair CODE",
		"",
		"Для отладки можно отправить /whoami.",
	}, "\n")
}

func whoamiMessage(chatID, userID, username string, paired bool) string {
	if username == "" {
		username = "-"
	}
	return strings.Join([]string{
		"Telegram identity:",
		"chat_id: " + chatID,
		"user_id: " + userID,
		"username: " + username,
		fmt.Sprintf("paired: %t", paired),
	}, "\n")
}

func defaultKeyboard() *replyKeyboardMarkup {
	return &replyKeyboardMarkup{
		Keyboard: [][]keyboardButton{
			{
				{Text: buttonReport},
				{Text: buttonCheckNow},
			},
			{
				{Text: buttonPing},
			},
		},
		ResizeKeyboard: true,
	}
}
