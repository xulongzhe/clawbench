#!/usr/bin/env bash
#
# ClawBench 开发调试启动脚本
#
# 用法:
#   ./dev.sh              # 后台启动（Go dev 后端 + Vite 热更新）
#   ./dev.sh --fg         # 前台启动
#   ./dev.sh --stop       # 停止后台进程
#   ./dev.sh --restart    # 重启
#

set -e

NAME="clawbench-dev"
BIN="./clawbench"
DEV_BACKEND_PID_FILE="/tmp/${NAME}-backend.pid"
DEV_PID_FILE="/tmp/${NAME}-vite.pid"

# Dev 模式端口（与正式版分离）
DEV_BACKEND_PORT=20002
DEV_FRONTEND_PORT=20001

get_watch_dir() {
    grep "^watch_dir:" "config.yaml" 2>/dev/null | awk '{print $2}' | tr -d '"' || echo ""
}

check_binary() {
    if [[ ! -f "$BIN" ]]; then
        echo "Binary not found, building..."
        if command -v go >/dev/null 2>&1; then
            go build -o "$BIN" .
        else
            echo "Error: Go not found and binary missing." >&2
            exit 1
        fi
    fi
}

_stop_dev() {
    for pfile in "$DEV_BACKEND_PID_FILE" "$DEV_PID_FILE"; do
        if [[ -f "$pfile" ]]; then
            local pid=$(cat "$pfile")
            if kill -0 "$pid" 2>/dev/null; then
                echo "Stopping $([[ "$pfile" == *backend* ]] && echo backend || echo Vite) (PID $pid)..."
                kill "$pid"
            fi
            rm -f "$pfile"
        fi
    done

    # Fallback: kill by port
    for port in $DEV_BACKEND_PORT $DEV_FRONTEND_PORT; do
        local pids=$(lsof -ti :$port 2>/dev/null)
        if [[ -n "$pids" ]]; then
            echo "Killing orphan process on port $port (PIDs: $pids)..."
            echo "$pids" | xargs kill -9 2>/dev/null || true
        fi
    done
}

start_dev() {
    # Kill existing dev processes first
    _stop_dev
    sleep 0.5

    check_binary

    local WATCH_DIR=$(get_watch_dir)
    echo "=== Starting $NAME (dev mode) ==="
    echo "  Binary:   $BIN"
    echo "  Backend:  http://localhost:$DEV_BACKEND_PORT"
    echo "  Frontend: http://localhost:$DEV_FRONTEND_PORT"
    echo "  DB:       ClawBench-dev.db (separate from release)"
    echo "  Watch:    ${WATCH_DIR:-default}"
    echo ""

    # Start Go backend in dev mode
    nohup $BIN --dev --port $DEV_BACKEND_PORT > /tmp/clawbench-dev-backend.log 2>&1 &
    echo $! > "$DEV_BACKEND_PID_FILE"
    disown $! 2>/dev/null

    sleep 0.3
    if ! kill -0 $(cat "$DEV_BACKEND_PID_FILE") 2>/dev/null; then
        echo "Failed to start dev backend." >&2
        rm -f "$DEV_BACKEND_PID_FILE"
        exit 1
    fi
    echo "Dev backend started (PID $(cat "$DEV_BACKEND_PID_FILE")) on port $DEV_BACKEND_PORT"

    # Start Vite dev server
    VITE_BACKEND_PORT=$DEV_BACKEND_PORT nohup npx vite --port $DEV_FRONTEND_PORT > /tmp/vite-dev.log 2>&1 &
    echo $! > "$DEV_PID_FILE"
    echo "Vite dev server started (PID $(cat "$DEV_PID_FILE")) on port $DEV_FRONTEND_PORT"
    echo ""
    echo "Open http://localhost:$DEV_FRONTEND_PORT in your browser"
    echo "Logs: /tmp/vite-dev.log  /tmp/clawbench-dev-backend.log"
}

# Parse arguments
ACTION="start"
FOREGROUND=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --fg)
            FOREGROUND=1
            ;;
        --stop)
            ACTION=stop
            ;;
        --restart)
            ACTION=restart
            ;;
        *)
            echo "未知参数: $1"
            echo "用法: $0 [--fg] [--stop] [--restart]"
            exit 1
            ;;
    esac
    shift
done

case "$ACTION" in
    stop)
        echo "Stopping dev processes..."
        _stop_dev
        echo "Done."
        ;;
    restart)
        echo "Restarting dev..."
        _stop_dev
        sleep 1
        start_dev
        ;;
    start)
        start_dev
        ;;
esac