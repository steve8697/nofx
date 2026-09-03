#!/bin/zsh
# 等 Docker 引擎起来后把 AETHERIS compose 拉起。
# 不会改 is_running：交易员只在数据库标记为运行时才会自动开仓。
set -euo pipefail
export PATH="/usr/local/bin:/opt/homebrew/bin:/Applications/Docker.app/Contents/Resources/bin:/usr/bin:/bin:/usr/sbin:/sbin"
cd /Users/huangjunyou/aetheris
log=/Users/huangjunyou/aetheris/logs/autostart.log
mkdir -p /Users/huangjunyou/aetheris/logs

{
  echo "---- $(date) ensure-aetheris-up ----"
  for i in {1..60}; do
    if docker info >/dev/null 2>&1; then
      echo "docker ready after ${i} tries"
      break
    fi
    echo "waiting for docker ($i)"
    sleep 5
  done
  if ! docker info >/dev/null 2>&1; then
    echo "docker never became ready"
    exit 1
  fi
  docker compose up -d
  docker compose ps
} >>"$log" 2>&1
