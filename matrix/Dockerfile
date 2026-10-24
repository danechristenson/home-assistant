# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM and the other ARGs from build.yaml,
# so they need no defaults here; defaults would duplicate the pinned versions)
ARG BUILD_FROM

# Element Web (the browser client) and the QR code library for the invite page
FROM alpine:3.24 AS web
ARG ELEMENT_VERSION
ARG QRCODE_VERSION
RUN wget -qO- "https://github.com/element-hq/element-web/releases/download/${ELEMENT_VERSION}/element-${ELEMENT_VERSION}.tar.gz" \
        | tar -xz -C /tmp \
    && mkdir -p /www/invite \
    && mv "/tmp/element-${ELEMENT_VERSION}" /www/element \
    && rm /www/element/config.sample.json \
    && wget -qO /www/invite/qrcode.js "https://cdn.jsdelivr.net/npm/qrcode-generator@${QRCODE_VERSION}/dist/qrcode.js"

FROM busybox:1.38-musl AS busybox

# The upstream image is distroless; add a static busybox for the shell and web server
FROM ${BUILD_FROM}
COPY --from=busybox /bin/busybox /bin/busybox
RUN ["/bin/busybox", "--install", "-s", "/bin"]
RUN mkdir -p -m 1777 /tmp
COPY --from=web /www /var/www
COPY rootfs /

COPY run.sh /run.sh
RUN chmod +x /run.sh /var/www/cgi-bin/session /var/www/cgi-bin/index.cgi /var/www/cgi-bin/password /usr/bin/call-permissions
ENTRYPOINT ["/run.sh"]
