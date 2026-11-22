# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM from build.yaml)
ARG BUILD_FROM

# ZoeyVid's NPMplus image (one image for amd64 and arm64), unchanged
FROM ${BUILD_FROM}
COPY run.sh /usr/local/bin/ha-run.sh
RUN chmod 0755 /usr/local/bin/ha-run.sh
# The image's own entrypoint is `tini -- entrypoint.sh`; run.sh hands over to it
ENTRYPOINT ["tini", "--", "/usr/local/bin/ha-run.sh"]
