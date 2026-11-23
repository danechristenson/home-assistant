# Node.js on Debian. Foundry itself is never in the image: it's licensed to
# each user, so run.sh downloads it from their own timed link into /data.
FROM node:26.13.1-trixie-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl jq unzip \
    && rm -rf /var/lib/apt/lists/*
COPY run.sh ingress.mjs /
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
