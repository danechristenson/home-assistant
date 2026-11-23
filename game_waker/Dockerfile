# Home Assistant's Alpine base, plus tcpdump to watch the ports and Python for
# the waker and its start page (standard library only)
FROM ghcr.io/home-assistant/base:3.22
RUN apk add --no-cache python3 tcpdump
COPY game_waker.py icon.png /
ENTRYPOINT ["python3", "-u", "/game_waker.py"]
