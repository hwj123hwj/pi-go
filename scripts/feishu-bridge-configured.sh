#!/bin/sh
set -eu

if [ -n "${FEISHU_APP_ID:-}" ] && [ -n "${FEISHU_APP_SECRET:-}" ]; then
	 exit 0
fi

if [ -s /root/.pi-go/feishu-credentials.json ]; then
	 exit 0
fi

echo "Feishu bridge is not configured; run /feishu setup or set FEISHU_APP_ID and FEISHU_APP_SECRET." >&2
exit 1
