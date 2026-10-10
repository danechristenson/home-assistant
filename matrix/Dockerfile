ARG BUILD_FROM

# Element Web, the browser client served through ingress
FROM alpine:3.22 AS element
ARG ELEMENT_VERSION
RUN wget -qO- "https://github.com/element-hq/element-web/releases/download/${ELEMENT_VERSION}/element-${ELEMENT_VERSION}.tar.gz" \
        | tar -xz -C /tmp \
    && mv "/tmp/element-${ELEMENT_VERSION}" /element \
    && rm /element/config.sample.json

FROM busybox:1.37-musl AS busybox

# The upstream image is distroless; add a static busybox for the shell and web server
FROM ${BUILD_FROM}
COPY --from=busybox /bin/busybox /bin/busybox
RUN ["/bin/busybox", "--install", "-s", "/bin"]
RUN mkdir -p -m 1777 /tmp
COPY --from=element /element /var/www/element

COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
