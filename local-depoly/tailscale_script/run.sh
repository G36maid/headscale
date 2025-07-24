#!/bin/sh
set -e

tailscaled &

sleep 2

tailscale up --login-server=http://${HS_HOSTNAME}:8080 --authkey="${TS_AUTHKEY}" --accept-dns=false

tail -f /dev/null
