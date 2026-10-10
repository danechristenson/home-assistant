ARG BUILD_FROM
FROM ${BUILD_FROM}

# Store all server data in the add-on's persistent /data volume
RUN sed -i 's|^data_dir = .*|data_dir = "/data"|' /home/vocechat-server/config/config.toml \
    && grep -q '^data_dir = "/data"$' /home/vocechat-server/config/config.toml

COPY run.sh /run.sh
RUN chmod +x /run.sh
ENTRYPOINT ["/run.sh"]
