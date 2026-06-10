# server-bot

Маленький Go-мониторинг для сайта/сервера.

Сейчас умеет:

- проверять HTTP targets из `configs/local.json`;
- отдавать `/health` и `/status`;
- отправлять Telegram-алерты, если цель перешла в `down` или восстановилась из `down` в `up`;
- отвечать в Telegram на команды и кнопки.

## Локальный запуск

```bash
go run ./cmd/bot
```

В другом терминале:

```bash
curl -sS http://127.0.0.1:8080/health
curl -sS http://127.0.0.1:8080/status
```

## Telegram

Telegram включается, если задан `TELEGRAM_BOT_TOKEN`.
`TELEGRAM_CHAT_ID` можно не задавать: тогда бот привяжет admin chat через `/pair CODE`.

```bash
export TELEGRAM_BOT_TOKEN="123456789:replace-me"
go run ./cmd/bot
```

При первом запуске без `TELEGRAM_CHAT_ID` бот напишет pairing code в server logs:

```text
telegram pairing required code=... expires_at=... state_path=data/state.json
```

После этого напиши боту:

```text
/pair CODE
```

Бот сохранит admin chat в `data/state.json`. Этот файл не коммитится в git.

Если хочешь задать chat id вручную, можно по-старому:

```bash
export TELEGRAM_CHAT_ID="123456789"
```

Токен, chat id и state-файл не лежат в `config.json`, чтобы не хранить секреты в git.

Telegram умеет отправлять алерты:

- цель стала `down`;
- цель восстановилась из `down` в `up`.

И отвечать на ручные команды:

- `/start` или `/help` - показать подсказку и кнопки;
- `/status` - показать последний отчет;
- `/check` - запустить проверки вручную и показать отчет;
- `/ping` - проверить связь с ботом;
- `/whoami` - показать текущие `chat_id` и `user_id`.

В чате также появятся кнопки:

- `Отчет`;
- `Проверить сейчас`;
- `Проверить соединение`.

## Docker

```bash
docker build -t server-bot:local .
docker run --rm -p 8080:8080 \
  --env-file .env \
  -v "$PWD/configs/local.json:/app/config.json:ro" \
  -v "$PWD/data:/app/data" \
  server-bot:local
```
