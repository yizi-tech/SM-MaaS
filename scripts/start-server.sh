#!/usr/bin/env bash
cd /opt/mass && set -a && . docker/.env && set +a
export DB_PORT=3306 MASS_FRONTEND_DIR=/opt/mass/frontend
exec setsid ./mass-server > /tmp/opencode/mass-server.log 2>&1 < /dev/null