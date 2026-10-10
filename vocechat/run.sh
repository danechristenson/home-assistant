#!/bin/sh
set -e
cd /home/vocechat-server
exec ./vocechat-server config/config.toml
