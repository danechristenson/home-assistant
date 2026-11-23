# lk-jwt-service: lets Matrix users into LiveKit calls (static binary)
FROM ghcr.io/element-hq/lk-jwt-service:0.10.0 AS jwt

FROM livekit/livekit-server:v1.13.11
COPY --from=jwt /lk-jwt-service /usr/local/bin/lk-jwt-service

COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
