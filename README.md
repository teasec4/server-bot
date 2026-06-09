# server-bot

Маленький Go-мониторинг для сайта/сервера.

Сейчас умеет:

- проверять HTTP targets из `configs/local.json`;
- отдавать `/health` и `/status`;
- отправлять Telegram-алерты, если цель перешла в `down` или восстановилась из `down` в `up`.

## Локальный запуск

```bash
go run ./cmd/bot -config configs/local.json
```

В другом терминале:

```bash
curl -sS http://127.0.0.1:8080/health
curl -sS http://127.0.0.1:8080/status
```

## Telegram

Telegram включается только если заданы обе переменные:

```bash
export TELEGRAM_BOT_TOKEN="123456789:replace-me"
export TELEGRAM_CHAT_ID="123456789"
go run ./cmd/bot -config configs/local.json
```

Токен и chat id не лежат в `config.json`, чтобы не хранить секреты в git.

Первый Telegram-шаг отправляет только алерты:

- цель стала `down`;
- цель восстановилась из `down` в `up`.

Команды вроде `/status` добавим отдельным шагом.

## Docker

```bash
docker build -t server-bot:local .
docker run --rm -p 8080:8080 \
  --env-file .env \
  -v "$PWD/configs/local.json:/app/config.json:ro" \
  server-bot:local
```
