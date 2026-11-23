# coturn, the TURN server
FROM coturn/coturn:4.19.2-alpine

# The upstream image runs as an unprivileged user; the add-on needs to read
# its options and write to /data and /share
USER root

COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
