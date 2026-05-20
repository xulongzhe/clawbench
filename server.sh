#!/usr/bin/env bash
#
# ClawBench 正式版启动脚本
#
# 用法:
#   ./server.sh              # 后台启动
#   ./server.sh --fg         # 前台启动
#   ./server.sh --stop       # 停止本项目的服务
#   ./server.sh --restart    # 重启
#

NAME="clawbench"
BIN="./$NAME"
CONFIG="config/config.yaml"
AUTO_PW_FILE=".clawbench/auto-password"

RELEASE_PORT=20000

# All runtime data under .clawbench/ (green portable deployment)
PID_FILE=".clawbench/server.pid"
LOG_FILE=".clawbench/server.log"

# --- Inline utility functions (from scripts/common.sh) ---

# Read watch_dir from config file; returns empty string if not found.
get_watch_dir() {
    local config="$1"
    grep "^watch_dir:" "$config" 2>/dev/null | awk '{print $2}' | tr -d '"' || echo ""
}

# Print auto-generated password if the file exists.
show_auto_password() {
    local auto_pw_file="$1"
    if [[ -f "$auto_pw_file" ]]; then
        local pw
        pw=$(cat "$auto_pw_file")
        echo "  Password: $pw (auto-generated, saved in $auto_pw_file)"
    fi
}

# Ensure the Go binary exists; build it if missing.
check_binary() {
    local bin="$1"
    if [[ ! -f "$bin" ]]; then
        echo "Binary not found, building..."
        if command -v go >/dev/null 2>&1; then
            local ver is_release build_time full_ver
            ver=$(git describe --tags --always 2>/dev/null || echo "dev")
            if git describe --tags --exact-match HEAD >/dev/null 2>&1; then
                is_release=true
            else
                is_release=false
            fi
            build_time=$(date +"%Y-%m-%d %H:%M:%S")
            if $is_release; then
                full_ver="$ver"
            else
                full_ver="$ver ($build_time)"
            fi
            go build -ldflags "-X clawbench/internal/version.Version=$full_ver" -o "$bin" ./cmd/server
        else
            echo "Error: Go not found and binary missing." >&2
            exit 1
        fi
    fi
}

# Stop processes by PID file and/or port.
_stop_servers() {
    local pid_file="$1"
    local port="$2"
    local name="${3:-server}"

    if [[ -n "$pid_file" && -f "$pid_file" ]]; then
        local pid
        pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            echo "Stopping $name (PID $pid)..."
            kill "$pid"
            sleep 1
            if kill -0 "$pid" 2>/dev/null; then
                kill -9 "$pid" 2>/dev/null
                sleep 1
            fi
        fi
        rm -f "$pid_file"
    fi

    if [[ -n "$port" ]]; then
        local pids=""
        if command -v ss >/dev/null 2>&1; then
            pids=$(ss -tlnp 2>/dev/null | grep ":$port " | grep -oP 'pid=\K[0-9]+' | sort -u | tr '\n' ' ')
        elif command -v netstat >/dev/null 2>&1; then
            pids=$(netstat -tlnp 2>/dev/null | grep ":$port " | grep -oP '\s[0-9]+/' | grep -oP '[0-9]+' | sort -u | tr '\n' ' ')
        fi
        if [[ -n "$pids" ]]; then
            echo "Killing orphan process on port $port (PIDs: $pids)..."
            echo "$pids" | xargs kill 2>/dev/null || true
            sleep 1
            if command -v ss >/dev/null 2>&1; then
                local remaining
                remaining=$(ss -tlnp 2>/dev/null | grep ":$port " | grep -oP 'pid=\K[0-9]+' | sort -u | tr '\n' ' ')
                if [[ -n "$remaining" ]]; then
                    echo "$remaining" | xargs kill -9 2>/dev/null || true
                    sleep 1
                fi
            fi
        fi

        local waited=0
        while [[ $waited -lt 5 ]]; do
            local bound=""
            if command -v ss >/dev/null 2>&1; then
                bound=$(ss -tlnp 2>/dev/null | grep ":$port ") || true
            fi
            if [[ -z "$bound" ]]; then
                break
            fi
            sleep 0.5
            waited=$((waited + 1))
        done
    fi
}

# --- End inline utilities ---

# Resolve effective port from config (fallback to default).
_resolve_port() {
    local port
    port=$(grep "^port:" "$CONFIG" 2>/dev/null | awk '{print $2}' | tr -d '"')
    echo "${port:-$RELEASE_PORT}"
}

EFFECTIVE_PORT=$(_resolve_port)

# Stop only this project's instance using project-local PID file.
_stop_release() {
    if [[ -f "$PID_FILE" ]]; then
        local pid
        pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo "Stopping $NAME (PID $pid)..."
            _stop_servers "$PID_FILE" "$EFFECTIVE_PORT" "release backend"
        else
            echo "Stale PID file, cleaning up."
            rm -f "$PID_FILE"
        fi
    else
        echo "No PID file found ($PID_FILE)."
    fi

    # Clear stale DuckDB lock files to resolve RAG lock conflicts
    local lock_file=".clawbench/rag.duckdb"
    if [[ -f "${lock_file}.lock" ]]; then
        echo "Removing stale DuckDB lock..."
        rm -f "${lock_file}.lock"
    fi
}

start_release() {
    _stop_release
    sleep 0.3

    check_binary "$BIN"

    # Ensure .clawbench directory exists
    mkdir -p .clawbench

    local WATCH_DIR
    WATCH_DIR=$(get_watch_dir "$CONFIG")
    echo "=== Starting $NAME (release) ==="
    echo "  Binary:   $BIN"
    echo "  Config:   $CONFIG"
    echo "  Port:     $EFFECTIVE_PORT"
    echo "  Watch:    ${WATCH_DIR:-default}"
    echo "  PIDFile:  $PID_FILE"
    echo "  Log:      $LOG_FILE"
    show_auto_password "$AUTO_PW_FILE"
    echo ""

    if [[ -n "$FOREGROUND" ]]; then
        echo "Open http://localhost:$EFFECTIVE_PORT in your browser"
        echo ""
        PORT=$EFFECTIVE_PORT CLAWBENCH_NO_SUPERVISOR=1 exec "$BIN"
    else
        PORT=$EFFECTIVE_PORT CLAWBENCH_NO_SUPERVISOR=1 nohup $BIN >> "$LOG_FILE" 2>&1 &
        echo $! > "$PID_FILE"
        disown $! 2>/dev/null

        sleep 0.5
        if kill -0 $(cat "$PID_FILE") 2>/dev/null; then
            echo "Started (PID $(cat "$PID_FILE")) on port $EFFECTIVE_PORT"
            echo "Log: $LOG_FILE"
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
            exit 1
            ;;
    esac
    shift
done

case "$ACTION" in
    stop)
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
