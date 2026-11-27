# The official farmOS image (Drupal on Apache). run.sh moves the site's
# directories into /data before Apache starts.
FROM farmos/farmos:4.0.13
# farmOS 4 images end with USER www-data, but run.sh needs root: it owns /data
# to www-data and runs drush as www-data itself, and Apache drops its workers
# to www-data as it did on farmOS 3
USER root
COPY run.sh provision_api.php forget_missing_modules.php /
RUN chmod +x /run.sh
# On SQLite (the add-on's database) a query condition of archived = FALSE
# matches nothing, though 0 does, so farmOS 4's dashboard metrics count no
# assets and its location hierarchy lists none. Compare with 0 and 1 instead.
# (Nothing to change once farmOS fixes it: then this does nothing.)
RUN files=$(grep -rlE "condition\('archived', (FALSE|TRUE)\)" /opt/drupal/web/profiles/farm --include=*.php || true); \
    if [ -n "$files" ]; then \
        sed -i -E "s/condition\('archived', FALSE\)/condition('archived', 0)/g; s/condition\('archived', TRUE\)/condition('archived', 1)/g" $files; \
    fi
CMD ["/run.sh"]
