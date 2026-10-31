# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM from build.yaml, so it needs no
# default here; a default would duplicate the pinned version)
ARG BUILD_FROM

# The official farmOS image (Drupal on Apache). run.sh moves the site's
# directories into /data before Apache starts.
FROM ${BUILD_FROM}
# farmOS 4 images end with USER www-data, but run.sh needs root: it owns /data
# to www-data and runs drush as www-data itself, and Apache drops its workers
# to www-data as it did on farmOS 3
USER root
COPY run.sh provision_api.php forget_missing_modules.php /
RUN chmod +x /run.sh
CMD ["/run.sh"]
