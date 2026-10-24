# check=skip=InvalidDefaultArgInFrom
# (Home Assistant always passes BUILD_FROM from build.yaml, so it needs no
# default here; a default would duplicate the pinned version)
ARG BUILD_FROM

# Home Assistant's Alpine base, plus tcpdump to watch the ports and Python for
# the waker and its start page (standard library only)
FROM ${BUILD_FROM}
RUN apk add --no-cache python3 tcpdump
COPY game_waker.py /game_waker.py
ENTRYPOINT ["python3", "-u", "/game_waker.py"]
