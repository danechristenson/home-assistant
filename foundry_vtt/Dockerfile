# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM from build.yaml, so it needs no
# default here; a default would duplicate the pinned version)
ARG BUILD_FROM

# Node.js on Debian. Foundry itself is never in the image: it's licensed to
# each user, so run.sh downloads it from their own timed link into /data.
FROM ${BUILD_FROM}
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl jq unzip \
    && rm -rf /var/lib/apt/lists/*
COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
