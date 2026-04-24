#!/usr/bin/env bash
#
# ClawBench 启动脚本
#
# 用法:
#   ./start.sh              # 后台启动 Go 后端
#   ./start.sh --fg         # 前台启动
#   ./start.sh --dev        # 开发模式（后台启动 Go 后端 + Vite 热更新服务器）
#   ./start.sh --port 8080  # 指定端口
#   ./start.sh --stop       # 停止发布版后台进程
#   ./start.sh --stop --dev # 停止开发版后台进程
#   ./start.sh --stop --all # 停止所有进程
#   ./start.sh --restart    # 重启
#

NAME="clawbench"
BIN="./$NAME"
PID_FILE="/tmp/${NAME}.pid"
DEV_PID_FILE="/tmp/${NAME}-dev.pid"
DEV_BACKEND_PID_FILE="/tmp/${NAME}-dev-backend.pid"
CONFIG="config.yaml"
PORT=""
DEV=""
FOREGROUND=""

# Release mode port (default)
RELEASE_PORT=20000

# Dev mode ports (separate from release to avoid conflicts)
DEV_BACKEND_PORT=20002
DEV_FRONTEND_PORT=20001

# 读取 config.yaml 中的 watch_dir
get_watch_dir() {
    grep "^watch_dir:" "$CONFIG" 2>/dev/null | awk '{print $2}' | tr -d '"' || echo ""
}

STOP_TARGET=""  # empty=release only, dev=dev only, all=all

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case "$1" in
        --fg)
            FOREGROUND=1
            ;;
        --dev)
            DEV=1
            ;;
        --stop)
            ACTION=stop
            ;;
        --all)
            STOP_TARGET=all
            ;;
        --restart)
            ACTION=restart
            ;;
        --port)
            PORT="$2"
            shift
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
    shift
done

# --stop --dev means stop dev only; --stop alone stops release only; --stop --all stops everything
if [[ "$ACTION" == "stop" ]]; then
    if [[ -n "$DEV" && "$STOP_TARGET" != "all" ]]; then
        STOP_TARGET=dev
    elif [[ -z "$STOP_TARGET" ]]; then
        STOP_TARGET=release
    fi
fi

# 停止或重启
stop_server() {
    if [[ "$STOP_TARGET" == "all" ]]; then
        # Stop everything
        _stop_release
        _stop_dev
    elif [[ "$STOP_TARGET" == "dev" ]]; then
        _stop_dev
    else
        _stop_release
    fi
    echo "Stopped."
}

_stop_release() {
    if [[ -f "$PID_FILE" ]]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo "Stopping release backend (PID $pid)..." 
            kill "$pid"
            rm -f "$PID_FILE"
        else
            rm -f "$PID_FILE"
        fi
    fi

    # Fallback: kill release binary by port
    local pids=$(lsof -ti :"${PORT:-$RELEASE_PORT}" 2>/dev/null)
    if [[ -n "$pids" ]]; then
        for pid in $pids; do
            echo "Killing orphan release process on port ${PORT:-$RELEASE_PORT} (PID $pid)..."
            kill "$pid" 2>/dev/null
        done
    fi
}

_stop_dev() {
    local dev_pids=("$DEV_BACKEND_PID_FILE" "$DEV_PID_FILE")
    local dev_names=("dev backend" "dev frontend (vite)")

    for i in "${!dev_pids[@]}"; do
        local pfile="${dev_pids[$i]}"
        local pname="${dev_names[$i]}"
        if [[ -f "$pfile" ]]; then
            local pid=$(cat "$pfile")
            if kill -0 "$pid" 2>/dev/null; then
                echo "Stopping $pname (PID $pid)..."
                kill "$pid"
                rm -f "$pfile"
            else
                rm -f "$pfile"
            fi
        fi
    done

    # Fallback: kill dev processes by port
    local backend_pids=$(lsof -ti :$DEV_BACKEND_PORT 2>/dev/null)
    if [[ -n "$backend_pids" ]]; then
        for pid in $backend_pids; do
            echo "Killing orphan dev backend on port $DEV_BACKEND_PORT (PID $pid)..."
            kill "$pid" 2>/dev/null
        done
    fi
    local vite_pids=$(lsof -ti :$DEV_FRONTEND_PORT 2>/dev/null)
    if [[ -n "$vite_pids" ]]; then
        for pid in $vite_pids; do
            echo "Killing orphan vite on port $DEV_FRONTEND_PORT (PID $pid)..."
            kill "$pid" 2>/dev/null
        done
    fi
}

# 按端口杀进程
kill_by_port() {
    if [[ -n "$DEV" ]]; then
        # Dev mode: kill dev ports
        local backend_pids=$(lsof -ti :$DEV_BACKEND_PORT 2>/dev/null)
        if [[ -n "$backend_pids" ]]; then
            echo "Killing process on dev port $DEV_BACKEND_PORT (PIDs: $backend_pids)..."
            echo "$backend_pids" | xargs kill -9 2>/dev/null
            sleep 0.3
        fi
        local vite_pids=$(lsof -ti :$DEV_FRONTEND_PORT 2>/dev/null)
        if [[ -n "$vite_pids" ]]; then
            echo "Killing process on dev port $DEV_FRONTEND_PORT (PIDs: $vite_pids)..."
            echo "$vite_pids" | xargs kill -9 2>/dev/null
            sleep 0.3
        fi
    else
        # Release mode: kill release port
        local target_port="${PORT:-$RELEASE_PORT}"
        local pids=$(lsof -ti :"$target_port" 2>/dev/null)
        if [[ -n "$pids" ]]; then
            echo "Killing process on port $target_port (PIDs: $pids)..."
            echo "$pids" | xargs kill -9 2>/dev/null
            sleep 0.3
        fi
    fi
}

# 检查二进制
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

# 启动
start_server() {
    kill_by_port
    check_binary

    WATCH_DIR=$(get_watch_dir)
    echo "=== Starting $NAME ==="
    echo "  Binary:   $BIN"
    echo "  Config:   $CONFIG"
    echo "  Watch:    ${WATCH_DIR:-default}"

    if [[ -n "$PORT" ]]; then
        echo "  Port:     $PORT"
    fi

    if [[ -n "$DEV" ]]; then
        echo "  Mode:     development (with Vite HMR, background)"
        echo "  Backend:  http://localhost:$DEV_BACKEND_PORT"
        echo "  Frontend: http://localhost:$DEV_FRONTEND_PORT"
        echo "  Database: ClawBench-dev.db (separate from release)"
        echo ""

        # Start Go backend in dev mode (--dev flag enables separate DB, debug logging)
        nohup $BIN --dev --port $DEV_BACKEND_PORT >> /tmp/clawbench-dev-backend.log 2>&1 &
        echo $! > "$DEV_BACKEND_PID_FILE"
        disown $! 2>/dev/null  # Prevent shell from sending SIGHUP on exit

        sleep 0.3
        if ! kill -0 $(cat "$DEV_BACKEND_PID_FILE") 2>/dev/null; then
            echo "Failed to start dev backend." >&2
            rm -f "$DEV_BACKEND_PID_FILE"
            exit 1
        fi
        echo "Dev backend started (PID $(cat "$DEV_BACKEND_PID_FILE")) on port $DEV_BACKEND_PORT"

        # Start Vite dev server (background), pass backend port via env
        VITE_BACKEND_PORT=$DEV_BACKEND_PORT nohup npx vite --port $DEV_FRONTEND_PORT > /tmp/vite-dev.log 2>&1 &
        echo $! > "$DEV_PID_FILE"
        echo "Vite dev server started (PID $(cat "$DEV_PID_FILE")) on port $DEV_FRONTEND_PORT"
        echo ""
        echo "Open http://localhost:$DEV_FRONTEND_PORT in your browser"
        echo "Logs: /tmp/vite-dev.log"
        return
    fi

    if [[ -n "$FOREGROUND" ]]; then
        echo ""
        echo "Open http://localhost:${PORT:-$RELEASE_PORT} in your browser"
        echo ""
        if [[ -n "$PORT" ]]; then
            PORT=$PORT exec "$BIN"
        else
            exec "$BIN"
        fi
    else
        echo "  Mode:     release (background)"
        echo "  Port:     ${PORT:-$RELEASE_PORT}"
        nohup $BIN >> /tmp/clawbench-release.log 2>&1 &
        echo $! > "$PID_FILE"
        disown $! 2>/dev/null
        sleep 0.5
        if kill -0 $(cat "$PID_FILE") 2>/dev/null; then
            echo "Started (PID $(cat $PID_FILE))"
        else
            echo "Failed to start." >&2
            rm -f "$PID_FILE"
            exit 1
        fi
    fi
}

# 执行
case "${ACTION:-start}" in
    stop)
        stop_server
        ;;
    restart)
        stop_server
        sleep 1
        start_server
        ;;
    start|*)
        start_server
        ;;
esac
