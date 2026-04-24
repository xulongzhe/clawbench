# ClawBench 启动脚本 (Windows PowerShell)
#
# 用法:
#   .\server.ps1              # 后台启动 Go 后端
#   .\server.ps1 -Foreground  # 前台启动
#   .\server.ps1 -Dev         # 开发模式（后台启动 Go 后端 + Vite 热更新服务器）
#   .\server.ps1 -Port 8080   # 指定端口
#   .\server.ps1 -Stop        # 停止发布版后台进程
#   .\server.ps1 -Stop -Dev   # 停止开发版后台进程
#   .\server.ps1 -Stop -All   # 停止所有进程
#   .\server.ps1 -Restart     # 重启

param(
    [switch]$Stop,
    [switch]$Dev,
    [switch]$Foreground,
    [switch]$All,
    [switch]$Restart,
    [int]$Port = 0
)

$NAME = "clawbench"
$BIN = ".\$NAME.exe"
$CONFIG = "config.yaml"

# Release mode port (default)
$RELEASE_PORT = 20000

# Dev mode ports (separate from release to avoid conflicts)
$DEV_BACKEND_PORT = 20002
$DEV_FRONTEND_PORT = 20001

# PID files stored in temp directory
$PID_FILE = Join-Path $env:TEMP "$NAME.pid"
$DEV_PID_FILE = Join-Path $env:TEMP "$NAME-dev.pid"
$DEV_BACKEND_PID_FILE = Join-Path $env:TEMP "$NAME-dev-backend.pid"

# Read watch_dir from config.yaml
function Get-WatchDir {
    if (Test-Path $CONFIG) {
        $line = Get-Content $CONFIG | Where-Object { $_ -match '^watch_dir:' } | Select-Object -First 1
        if ($line) {
            $value = ($line -split ':', 2)[1].Trim().Trim('"').Trim("'")
            # Expand ~ to user home
            if ($value -eq '~') {
                $value = $env:USERPROFILE
            } elseif ($value.StartsWith('~\') -or $value.StartsWith('~/')) {
                $value = Join-Path $env:USERPROFILE $value.Substring(2)
            }
            return $value
        }
    }
    return ""
}

function Stop-Release {
    if (Test-Path $PID_FILE) {
        $pid = Get-Content $PID_FILE -Raw
        $pid = $pid.Trim()
        $proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
        if ($proc) {
            Write-Host "Stopping release backend (PID $pid)..."
            Stop-Process -Id $pid -Force
        }
        Remove-Item $PID_FILE -Force -ErrorAction SilentlyContinue
    }

    # Fallback: kill by port
    $targetPort = if ($Port -gt 0) { $Port } else { $RELEASE_PORT }
    $connections = Get-NetTCPConnection -LocalPort $targetPort -ErrorAction SilentlyContinue
    if ($connections) {
        $pids = $connections | Select-Object -ExpandProperty OwningProcess -Unique
        foreach ($p in $pids) {
            Write-Host "Killing orphan release process on port $targetPort (PID $p)..."
            Stop-Process -Id $p -Force -ErrorAction SilentlyContinue
        }
    }
}

function Stop-Dev {
    @($DEV_BACKEND_PID_FILE, $DEV_PID_FILE) | ForEach-Object {
        $pfile = $_
        if (Test-Path $pfile) {
            $pid = (Get-Content $pfile -Raw).Trim()
            $proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
            if ($proc) {
                $pname = if ($pfile -eq $DEV_BACKEND_PID_FILE) { "dev backend" } else { "dev frontend (vite)" }
                Write-Host "Stopping $pname (PID $pid)..."
                Stop-Process -Id $pid -Force
            }
            Remove-Item $pfile -Force -ErrorAction SilentlyContinue
        }
    }

    # Fallback: kill by port
    $backendConns = Get-NetTCPConnection -LocalPort $DEV_BACKEND_PORT -ErrorAction SilentlyContinue
    if ($backendConns) {
        $pids = $backendConns | Select-Object -ExpandProperty OwningProcess -Unique
        foreach ($p in $pids) {
            Write-Host "Killing orphan dev backend on port $DEV_BACKEND_PORT (PID $p)..."
            Stop-Process -Id $p -Force -ErrorAction SilentlyContinue
        }
    }
    $viteConns = Get-NetTCPConnection -LocalPort $DEV_FRONTEND_PORT -ErrorAction SilentlyContinue
    if ($viteConns) {
        $pids = $viteConns | Select-Object -ExpandProperty OwningProcess -Unique
        foreach ($p in $pids) {
            Write-Host "Killing orphan vite on port $DEV_FRONTEND_PORT (PID $p)..."
            Stop-Process -Id $p -Force -ErrorAction SilentlyContinue
        }
    }
}

function Stop-Server {
    if ($All) {
        Stop-Release
        Stop-Dev
    } elseif ($Dev) {
        Stop-Dev
    } else {
        Stop-Release
    }
    Write-Host "Stopped."
}

function Kill-ByPort {
    if ($Dev) {
        $backendConns = Get-NetTCPConnection -LocalPort $DEV_BACKEND_PORT -ErrorAction SilentlyContinue
        if ($backendConns) {
            $pids = $backendConns | Select-Object -ExpandProperty OwningProcess -Unique
            Write-Host "Killing process on dev port $DEV_BACKEND_PORT (PIDs: $($pids -join ','))..."
            $pids | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
            Start-Sleep -Milliseconds 300
        }
        $viteConns = Get-NetTCPConnection -LocalPort $DEV_FRONTEND_PORT -ErrorAction SilentlyContinue
        if ($viteConns) {
            $pids = $viteConns | Select-Object -ExpandProperty OwningProcess -Unique
            Write-Host "Killing process on dev port $DEV_FRONTEND_PORT (PIDs: $($pids -join ','))..."
            $pids | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
            Start-Sleep -Milliseconds 300
        }
    } else {
        $targetPort = if ($Port -gt 0) { $Port } else { $RELEASE_PORT }
        $connections = Get-NetTCPConnection -LocalPort $targetPort -ErrorAction SilentlyContinue
        if ($connections) {
            $pids = $connections | Select-Object -ExpandProperty OwningProcess -Unique
            Write-Host "Killing process on port $targetPort (PIDs: $($pids -join ','))..."
            $pids | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
            Start-Sleep -Milliseconds 300
        }
    }
}

function Check-Binary {
    if (-not (Test-Path $BIN)) {
        Write-Host "Binary not found, building..."
        if (Get-Command go -ErrorAction SilentlyContinue) {
            go build -o $BIN .
        } else {
            Write-Host "Error: Go not found and binary missing." -ForegroundColor Red
            exit 1
        }
    }
}

function Start-Server {
    Kill-ByPort
    Check-Binary

    $WATCH_DIR = Get-WatchDir
    Write-Host "=== Starting $NAME ==="
    Write-Host "  Binary:   $BIN"
    Write-Host "  Config:   $CONFIG"
    Write-Host "  Watch:    $(if ($WATCH_DIR) { $WATCH_DIR } else { 'default' })"

    if ($Port -gt 0) {
        Write-Host "  Port:     $Port"
    }

    if ($Dev) {
        Write-Host "  Mode:     development (with Vite HMR, background)"
        Write-Host "  Backend:  http://localhost:$DEV_BACKEND_PORT"
        Write-Host "  Frontend: http://localhost:$DEV_FRONTEND_PORT"
        Write-Host ""

        # Start Go backend in dev mode
        $logFile = Join-Path $env:TEMP "clawbench-dev-backend.log"
        $proc = Start-Process -FilePath $BIN -ArgumentList "--dev", "--port", $DEV_BACKEND_PORT -RedirectStandardOutput $logFile -RedirectStandardError (Join-Path $env:TEMP "clawbench-dev-backend-err.log") -NoNewWindow -PassThru
        $proc.Id | Out-File $DEV_BACKEND_PID_FILE -Encoding utf8

        Start-Sleep -Milliseconds 300
        $checkProc = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
        if (-not $checkProc) {
            Write-Host "Failed to start dev backend." -ForegroundColor Red
            Remove-Item $DEV_BACKEND_PID_FILE -Force -ErrorAction SilentlyContinue
            exit 1
        }
        Write-Host "Dev backend started (PID $($proc.Id)) on port $DEV_BACKEND_PORT"

        # Start Vite dev server
        $viteLog = Join-Path $env:TEMP "vite-dev.log"
        $env:VITE_BACKEND_PORT = $DEV_BACKEND_PORT
        $viteProc = Start-Process -FilePath "npx" -ArgumentList "vite", "--port", $DEV_FRONTEND_PORT -RedirectStandardOutput $viteLog -RedirectStandardError (Join-Path $env:TEMP "vite-dev-err.log") -NoNewWindow -PassThru
        $viteProc.Id | Out-File $DEV_PID_FILE -Encoding utf8
        Write-Host "Vite dev server started (PID $($viteProc.Id)) on port $DEV_FRONTEND_PORT"
        Write-Host ""
        Write-Host "Open http://localhost:$DEV_FRONTEND_PORT in your browser"
        Write-Host "Logs: $viteLog"
        return
    }

    if ($Foreground) {
        $targetPort = if ($Port -gt 0) { $Port } else { $RELEASE_PORT }
        Write-Host ""
        Write-Host "Open http://localhost:$targetPort in your browser"
        Write-Host ""
        if ($Port -gt 0) {
            & $BIN --port $Port
        } else {
            & $BIN
        }
    } else {
        $targetPort = if ($Port -gt 0) { $Port } else { $RELEASE_PORT }
        Write-Host "  Mode:     release (background)"
        Write-Host "  Port:     $targetPort"

        $logFile = Join-Path $env:TEMP "clawbench-release.log"
        $args = @()
        if ($Port -gt 0) {
            $args += @("--port", $Port)
        }
        $proc = Start-Process -FilePath $BIN -ArgumentList $args -RedirectStandardOutput $logFile -RedirectStandardError (Join-Path $env:TEMP "clawbench-release-err.log") -NoNewWindow -PassThru
        $proc.Id | Out-File $PID_FILE -Encoding utf8

        Start-Sleep -Milliseconds 500
        $checkProc = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
        if ($checkProc) {
            Write-Host "Started (PID $($proc.Id))"
        } else {
            Write-Host "Failed to start." -ForegroundColor Red
            Remove-Item $PID_FILE -Force -ErrorAction SilentlyContinue
            exit 1
        }
    }
}

# Execute
if ($Restart) {
    if ($Dev) { Stop-Dev } else { Stop-Release }
    Start-Sleep -Seconds 1
    Start-Server
} elseif ($Stop) {
    Stop-Server
} else {
    Start-Server
}
