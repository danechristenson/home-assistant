# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM and the other ARGs from build.yaml,
# so they need no defaults here; defaults would duplicate the pinned versions)
ARG BUILD_FROM
FROM ${BUILD_FROM}

# The upstream image runs as an unprivileged user; the add-on needs to read
# its options and write to /data and /share
USER root

COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
