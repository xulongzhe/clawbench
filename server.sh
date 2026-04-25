#!/usr/bin/env bash
#
# ClawBench 正式版启动脚本
#
# 用法:
#   ./server.sh              # 后台启动
#   ./server.sh --fg         # 前台启动
#   ./server.sh --port 8080  # 指定端口
#   ./server.sh --stop       # 停止后台进程
#   ./server.sh --restart    # 重启
#

NAME="clawbench"
BIN="./$NAME"
PID_FILE="/tmp/${NAME}.pid"
CONFIG="config.yaml"

RELEASE_PORT=20000

get_watch_dir() {
    grep "^watch_dir:" "$CONFIG" 2>/dev/null | awk '{print $2}' | tr -d '"' || echo ""
}

check_binary() {
    if [[ ! -f "$BIN" ]]; then
        echo "Binary not found, building..."
        if command -v go >/dev/null 2>&1; then
            go build -o "$BIN" ./cmd/server
        else
            echo "Error: Go not found and binary missing." >&2
            exit 1
        fi
    fi
}

_stop_release() {
    if [[ -f "$PID_FILE" ]]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo "Stopping release backend (PID $pid)..."
            kill "$pid"
        fi
        rm -f "$PID_FILE"
    fi

    # Fallback: kill by port
    local pids=$(lsof -ti :${PORT:-$RELEASE_PORT} 2>/dev/null)
    if [[ -n "$pids" ]]; then
        echo "Killing orphan process on port ${PORT:-$RELEASE_PORT} (PIDs: $pids)..."
        echo "$pids" | xargs kill 2>/dev/null || true
    fi
}

start_release() {
    _stop_release
    sleep 0.3

    check_binary

    local WATCH_DIR=$(get_watch_dir)
    echo "=== Starting $NAME (release) ==="
    echo "  Binary:   $BIN"
    echo "  Config:   $CONFIG"
    echo "  Port:     ${PORT:-$RELEASE_PORT}"
    echo "  Watch:    ${WATCH_DIR:-default}"
    echo ""

    if [[ -n "$FOREGROUND" ]]; then
        echo "Open http://localhost:${PORT:-$RELEASE_PORT} in your browser"
        echo ""
        if [[ -n "$PORT" ]]; then
            PORT=$PORT exec "$BIN"
        else
            exec "$BIN"
        fi
    else
        nohup $BIN >> /tmp/clawbench-release.log 2>&1 &
        echo $! > "$PID_FILE"
        disown $! 2>/dev/null

        sleep 0.5
        if kill -0 $(cat "$PID_FILE") 2>/dev/null; then
            echo "Started (PID $(cat "$PID_FILE")) on port ${PORT:-$RELEASE_PORT}"
            echo "Log: /tmp/clawbench-release.log"
        else
            echo "Failed to start." >&2
            rm -f "$PID_FILE"
            exit 1
        fi
    fi
}

# Parse arguments
ACTION="start"
FOREGROUND=""
PORT=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --fg)
            FOREGROUND=1
            ;;
        --port)
            PORT="$2"
            shift
            ;;
        --stop)
            ACTION=stop
            ;;
        --restart)
            ACTION=restart
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
    shift
done

case "$ACTION" in
    stop)
        echo "Stopping release..."
        _stop_release
        echo "Done."
        ;;
    restart)
        _stop_release
        sleep 1
        start_release
        ;;
    start)
        start_release
        ;;
esac