# ZoeyVid's NPMplus image (amd64 and arm64), unchanged. Pinned by tag and
# digest, so Home Assistant builds exactly what the smoke test built; the
# daily update moves both.
FROM docker.io/zoeyvid/npmplus:2026-11-18-r1@sha256:de97158ec24f463ce2b7fe6bbac58afe0ee9f4e2eb6beed380b9af041b1abcf6

# No profile pictures fetched from gravatar.com
ENV DISABLE_GRAVATAR=true

# One-time import from another NPMplus (see run.sh); the image's own
# entrypoint is `tini -- entrypoint.sh`, which run.sh hands over to
COPY run.sh /usr/local/bin/ha-run.sh
RUN chmod 0755 /usr/local/bin/ha-run.sh
ENTRYPOINT ["tini", "--", "/usr/local/bin/ha-run.sh"]
