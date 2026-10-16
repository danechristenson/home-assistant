# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM and the other ARGs from build.yaml,
# so they need no defaults here; defaults would duplicate the pinned versions)
ARG BUILD_FROM
ARG GO_VERSION

# The bot: one binary, built with mautrix-go's pure-Go encryption (goolm)
FROM golang:${GO_VERSION}-alpine AS build
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=1 go build -tags goolm -trimpath -ldflags "-s -w" -o /matrix-helper-bot .

FROM ${BUILD_FROM}
COPY --from=build /matrix-helper-bot /usr/bin/matrix-helper-bot
ENTRYPOINT ["/usr/bin/matrix-helper-bot"]
