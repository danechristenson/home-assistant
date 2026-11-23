# Lets the Matrix Helper Bot add players (static binary, standard library only)
FROM golang:1.27-alpine AS botapi
WORKDIR /src
COPY botapi/go.mod botapi/*.go botapi/panel.html ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /minecraft-bot-api .

# itzg/minecraft-bedrock-server: downloads and runs Mojang's Bedrock server
FROM itzg/minecraft-bedrock-server:2026.11.1
# The Bedrock version to download, pinned so updates are tested first; the
# image's own default is "latest"
ENV VERSION=1.26.66.1
COPY --from=botapi /minecraft-bot-api /usr/local/bin/minecraft-bot-api
COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
