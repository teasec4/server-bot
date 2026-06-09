FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server-bot ./cmd/bot

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
	&& adduser -D -H -u 10001 serverbot
USER serverbot

WORKDIR /app
COPY --from=build /out/server-bot /usr/local/bin/server-bot

EXPOSE 8080
ENTRYPOINT ["server-bot"]
CMD ["-config", "/app/config.json"]
