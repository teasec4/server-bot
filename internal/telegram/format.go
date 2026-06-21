package telegram

import (
	"fmt"
	"strings"

	"server-bot/internal/monitor"
)

func FormatTargetEvent(event monitor.TargetEvent) string {
	target := event.Target
	result := target.LastResult

	var b strings.Builder
	if event.CurrentState == "down" {
		b.WriteString("🚨 Цель недоступна\n")
	} else {
		b.WriteString("✅ Цель восстановилась\n")
	}

	b.WriteString(target.Name)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s → %s", stateLabel(event.PreviousState), stateLabel(event.CurrentState)))
	b.WriteString("\n")
	b.WriteString(target.URL)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Ошибок подряд: %d/%d", target.ConsecutiveFailures, target.FailureThreshold))

	if result != nil {
		b.WriteString(fmt.Sprintf("  |  %dms", result.DurationMS))
		if result.HTTPStatus != 0 {
			b.WriteString(fmt.Sprintf("  |  HTTP %d", result.HTTPStatus))
		}
		if result.Error != "" {
			b.WriteString(fmt.Sprintf("\n%s", result.Error))
		} else if result.Description != "" {
			b.WriteString(fmt.Sprintf("  |  %s", result.Description))
		}
	}

	return strings.TrimSpace(b.String())
}

func FormatSnapshot(snapshot monitor.Snapshot) string {
	var b strings.Builder

	// Заголовок с датой в читаемом формате
	b.WriteString("📊 Отчёт server-bot\n")
	b.WriteString(snapshot.GeneratedAt.Format("2 Jan 2006 15:04:05"))
	b.WriteString("\n")

	// Сводка по целям: ✅ 2/2
	if len(snapshot.Targets) > 0 {
		up := 0
		for _, t := range snapshot.Targets {
			if t.State == "up" {
				up++
			}
		}
		b.WriteString(fmt.Sprintf("━━ %d/%d доступно\n", up, len(snapshot.Targets)))
	}

	// Таблица целей
	if len(snapshot.Targets) == 0 {
		b.WriteString("\nНет настроенных проверок\n")
	} else {
		b.WriteString("\n")
		for _, t := range snapshot.Targets {
			b.WriteString(stateIcon(t.State))
			b.WriteString(" ")
			b.WriteString(t.Name)
			b.WriteByte('\n')
			if t.LastResult != nil {
				b.WriteString("   ")
				b.WriteString(fmt.Sprintf("%dms", t.LastResult.DurationMS))
				if t.LastResult.HTTPStatus != 0 {
					b.WriteString(fmt.Sprintf("  HTTP %d", t.LastResult.HTTPStatus))
				}
				if t.LastResult.Error != "" {
					b.WriteString(fmt.Sprintf("  ⚡%s", t.LastResult.Error))
				}
				b.WriteByte('\n')
			}
		}
	}

	// Даты оплат
	if len(snapshot.Renewals) > 0 {
		b.WriteString("\n📅 Оплаты\n")
		for _, r := range snapshot.Renewals {
			b.WriteString(renewalIcon(r.State))
			b.WriteString(" ")
			b.WriteString(r.Name)
			b.WriteString(" — ")
			b.WriteString(r.DueDate)
			switch r.State {
			case "expired":
				b.WriteString(fmt.Sprintf(" (просрочено на %d дн.)", -r.DaysLeft))
			case "warning":
				b.WriteString(fmt.Sprintf(" (через %d дн.)", r.DaysLeft))
			default:
				b.WriteString(fmt.Sprintf(" (ещё %d дн.)", r.DaysLeft))
			}
			b.WriteByte('\n')
		}
	}

	return strings.TrimSpace(b.String())
}

func stateIcon(state string) string {
	switch state {
	case "up":
		return "🟢"
	case "down":
		return "🔴"
	case "suspect":
		return "🟡"
	default:
		return "⚪"
	}
}

func stateLabel(state string) string {
	switch state {
	case "up":
		return "🟢 доступна"
	case "down":
		return "🔴 недоступна"
	case "suspect":
		return "🟡 подозрение"
	default:
		return "⚪ ожидание"
	}
}

func renewalIcon(state string) string {
	switch state {
	case "expired":
		return "🔴"
	case "warning":
		return "🟡"
	default:
		return "🟢"
	}
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
