# The bot: one binary, built with mautrix-go's pure-Go encryption (goolm)
FROM golang:1.27-alpine AS build
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=1 go build -tags goolm -trimpath -ldflags "-s -w" -o /matrix-helper-bot .

FROM ghcr.io/home-assistant/base:3.24
COPY --from=build /matrix-helper-bot /usr/bin/matrix-helper-bot
ENTRYPOINT ["/usr/bin/matrix-helper-bot"]
