ARG BUILD_FROM
ARG LK_JWT_VERSION

# lk-jwt-service: lets Matrix users into LiveKit calls (static binary)
FROM ghcr.io/element-hq/lk-jwt-service:${LK_JWT_VERSION} AS jwt

FROM ${BUILD_FROM}
COPY --from=jwt /lk-jwt-service /usr/local/bin/lk-jwt-service

COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
