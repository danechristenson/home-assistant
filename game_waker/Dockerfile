# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM from build.yaml, so it needs no
# default here; a default would duplicate the pinned version)
ARG BUILD_FROM

# Home Assistant's Alpine base (bash, curl, jq) plus tcpdump to watch the ports
FROM ${BUILD_FROM}
RUN apk add --no-cache tcpdump
COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
