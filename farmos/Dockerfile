# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM from build.yaml, so it needs no
# default here; a default would duplicate the pinned version)
ARG BUILD_FROM

# The official farmOS image (Drupal on Apache). run.sh moves the site's
# directories into /data before Apache starts.
FROM ${BUILD_FROM}
COPY run.sh provision_api.php /
RUN chmod +x /run.sh
CMD ["/run.sh"]
