# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM and the other ARGs from build.yaml,
# so they need no defaults here; defaults would duplicate the pinned versions)
ARG BUILD_FROM
ARG GO_VERSION

# Lets the Matrix Helper Bot add players (static binary, standard library only)
FROM golang:${GO_VERSION}-alpine AS botapi
WORKDIR /src
COPY botapi/go.mod botapi/*.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /minecraft-bot-api .

# itzg/minecraft-bedrock-server: downloads and runs Mojang's Bedrock server
FROM ${BUILD_FROM}
ARG BEDROCK_VERSION
# The Bedrock version to download, pinned so updates are tested first; the
# image's own default is "latest"
ENV VERSION=${BEDROCK_VERSION}
COPY --from=botapi /minecraft-bot-api /usr/local/bin/minecraft-bot-api
COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
