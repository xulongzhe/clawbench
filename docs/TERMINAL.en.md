# Web Terminal User Guide

ClawBench's web terminal is built on PTY + WebSocket + xterm.js, giving you a near-native terminal experience directly in your browser or mobile app. This document covers all terminal features including touch gestures, virtual keys, quick commands, and more.

---

## Table of Contents

- [Interactive Terminal](#interactive-terminal)
- [Concurrent Sessions](#concurrent-sessions)
- [Virtual Key Toolbar](#virtual-key-toolbar)
- [Touch Gestures](#touch-gestures)
- [Quick Commands](#quick-commands)
- [Font & Viewport](#font--viewport)
- [Connection & Reconnection](#connection--reconnection)
- [Directory Detection & Rebuild](#directory-detection--rebuild)
- [Android Volume Keys](#android-volume-keys)
- [Configuration](#configuration)

---

## Interactive Terminal

The terminal connects to a server-side PTY (pseudo-terminal) via WebSocket. All input is sent to the PTY instantly, and output is rendered in real time.

- **Shell Detection**: Linux/macOS uses the `$SHELL` environment variable (falls back to `/bin/sh`); Windows checks `pwsh` → `powershell` → `cmd.exe` in order
- **Cursor Blink**: Enabled for a native terminal feel
- **Line Ending**: `convertEol` enabled — `\n` is automatically converted to `\r\n`
- **Scrollback**: 5000 lines of scrollback history
- **Clickable URLs**: URLs in terminal output are clickable
- **Right-Click Select Word**: Right-click auto-selects the word under the cursor
- **Themes**: Light (Catppuccin Latte) and Dark (Catppuccin Mocha) themes, auto-switching with system preference

## Concurrent Sessions

Each terminal client gets an independent PTY session with no interference.

- **Independent Sessions**: Each new connection creates a separate PTY session with a unique 8-byte hex session ID
- **Session Limit**: Default maximum of 10 concurrent sessions (configurable); an error is shown when the limit is reached
- **Isolated State**: Each session has its own PTY, ring buffer, idle timer, and WebSocket connection
- **Auto Cleanup**: Sessions are automatically removed when the PTY process exits

## Virtual Key Toolbar

A scrollable virtual key toolbar sits below the terminal viewport, optimized for mobile input. Keys are grouped by function with vertical dividers between groups:

### Gesture Toggle

The hand icon button on the far left (outside the scroll area) toggles touch gesture mode on/off.

### Modifier Group

| Key | Description |
|-----|-------------|
| **Ctl** | Ctrl modifier |
| **Alt** | Alt modifier |
| **Shift** | Shift modifier (icon display) |

**Three-State Toggle**:

1. **Inactive** → tap → **Once** (auto-clears after next keypress)
2. **Once** → tap → **Inactive**
3. **Once** → long-press → **Locked** (persists until tapped again)
4. **Locked** → tap → **Inactive**

Visual: Active background for once/locked states; locked state adds an inset bottom border.

**Modifier Input Processing**:

- Ctrl + A-Z: Sends `\x01` through `\x1a`
- Ctrl + `[` `\` `]` `@` `^` `_`: Sends corresponding control characters
- Alt + any single char: Sends `\x1b` + char (ESC prefix)
- Shift + Tab: Sends `\x1b[Z`
- After processing, once modifiers auto-clear; locked modifiers persist

### Shortcuts Group

| Key | Sends | Description |
|-----|-------|-------------|
| **^C** | `\x03` | Interrupt current command |
| **^Z** | `\x1a` | Suspend current process |

### Navigation Group

| Key | Sends | Description |
|-----|-------|-------------|
| **Home** | `\x1b[H` | Move cursor to line start |
| **End** | `\x1b[F` | Move cursor to line end |
| **PgUp** | `\x1b[5~` | Page up (hidden when gestures enabled) |
| **PgDn** | `\x1b[6~` | Page down (hidden when gestures enabled) |

### Arrow Keys Group

| Key | Description |
|-----|-------------|
| ↑ ↓ ← → | Four arrow keys (entire group hidden when gestures enabled) |

### Symbols Group

| Key | Description |
|-----|-------------|
| / - \| _ ~ | Common special characters, direct input |

### Actions Group

| Key | Description |
|-----|-------------|
| **Quick Commands** ⚡ | Popup menu listing visible commands; tap to execute |
| **Copy Output** 📋 | Copy all non-empty lines from xterm.js buffer to clipboard |
| **Rebuild Session** 🔄 | Close current PTY and create new session in current CWD |

## Touch Gestures

The terminal supports Termius-style touch gestures, toggled via the gesture button in the toolbar.

### When Gestures Are ENABLED

| Gesture | Action | Details |
|---------|--------|---------|
| **Single-finger swipe** | Arrow key | Swipe 30px+ in any direction: left/right/up/down sends the corresponding arrow key |
| **Hold direction** | Auto-repeat arrow keys | After swipe, hold direction for 500ms, then arrow key repeats every 150ms |
| **Long-press** | Esc | Hold finger still for 500ms without moving more than 10px |
| **Double-tap** | Tab | Two taps within 300ms and 20px of each other (triple-tap is prevented) |
| **Two-finger pinch** | Zoom font size | Pinch in/out changes font size by 1px per 10px of accumulated delta; range 8–28px |
| **Two-finger vertical swipe** | PgUp / PgDn | Both fingers move vertically in the same direction 30px+; upward = PgUp, downward = PgDn |

**Gesture hint overlay**: When a gesture fires, a large semi-transparent symbol (arrow, Esc, Tab, page symbol) appears in the center of the terminal for 600ms with fade-in/fade-out animation.

**Key hiding**: When gestures are enabled, Esc, Tab, arrow keys, and PgUp/PgDn are hidden from the virtual key toolbar (these actions are handled by gestures).

**Context menu suppression**: When gestures are enabled, long-press does not trigger the native select/copy menu.

### When Gestures Are DISABLED

- **Single-finger vertical drag**: Scrolls the xterm.js viewport (`term.scrollLines()`) without sending input to the PTY
- This is necessary because on mobile browsers, the xterm.js scrollable element is not the touch target
- **Long-press**: Opens the native select/copy menu as expected

## Quick Commands

Quick commands are user-defined terminal command shortcuts that can be executed with one tap.

### Data Fields

| Field | Description |
|-------|-------------|
| Label | Command name (max 100 characters) |
| Command | Actual command to execute (max 4096 characters) |
| Hidden | If enabled, the command is not shown in the popup menu |
| Auto-execute | If enabled, the command runs automatically on every connect/reconnect (at most one command can have this) |
| Sort Order | Drag reorder weight |

### Auto-Execute

- At most one command can have auto-execute enabled (enforced by a unique database index)
- Setting a new auto-execute command automatically clears the previous one
- On every terminal connect or reconnect, if an auto-execute command exists, it is sent to the PTY immediately

### Management UI

- Tap the ⚡ icon in the virtual key toolbar to open the command popup menu
- Tap "Edit Commands" to open the management dialog:
  - Drag reorder (≡ drag handle)
  - Inline delete confirmation
  - Create/edit modal: label, command, hidden toggle, auto-execute toggle
  - Auto-execute commands show a ⚡ badge; hidden commands show an eye-off badge and dimmed row
- Drag reorder uses optimistic update with rollback on API failure

## Font & Viewport

### Font Settings

- **Font Family**: `JetBrains Mono`, `Fira Code`, `Cascadia Code`, monospace fallback
- **Default Size**: 12px
- **Size Range**: 8–28px
- **Persistence**: Font size is stored in `localStorage`

### Font Size Adjustment

| Method | Action |
|--------|--------|
| **Desktop** | Ctrl+Wheel (or Cmd+Wheel) on the terminal area |
| **Mobile** | Two-finger pinch gesture |
| **Header number** | Click the font size number in the header to reset to default (12px) |

### Keyboard Avoidance (Mobile)

- Uses `window.visualViewport` to detect soft keyboard height
- Keyboard height is applied as CSS variable `--keyboard-height` on the panel
- FitAddon `fit()` calls are debounced by 100ms to prevent duplicate prompt lines during keyboard slide-up animation
- Bottom safe area padding: `padding-bottom: max(6px, env(safe-area-inset-bottom))`

### Scrollbar

Custom 2px thin scrollbar indicator overrides xterm.js default scrollbar styles.

## Connection & Reconnection

### Connection Status

Indicated by a status dot in the header:

| Color | Status |
|-------|--------|
| 🟢 Green | Connected |
| 🟡 Yellow (blinking) | Connecting / Reconnecting |
| ⚪ Gray | Disconnected / Error |

### Reconnection

- Automatic reconnection on WebSocket disconnect, up to 3 attempts
- Increasing delay: `2000ms × attempt number`
- Reconnects with stored `sessionId`; if session is still alive, client reattaches
- If session no longer exists, a new session is created

### Client Replacement

When a new client connects to a session that already has a connected client, the old client is kicked with custom close code `4001`. The frontend recognizes this code and **does not auto-reconnect**, preventing an infinite kick-reconnect loop.

### Fatal Errors

The following error codes block reconnection and show an error overlay:

| Error Code | Description |
|------------|-------------|
| `terminal_disabled` | Terminal feature is disabled (no reconnect button) |
| `shell_start_failed` | Failed to start shell (reconnect available) |
| `session_limit` | Max concurrent sessions reached (no reconnect button) |

### Ring Buffer & Replay

- PTY output is written to a ring buffer in real time while being sent via WebSocket
- Three-level memory protection:
  - Per-line byte cap (default 64KB); lines exceeding this are truncated with a `[truncated]` marker
  - Line count cap (default 2000 lines); oldest lines are evicted when full
  - Total memory cap (default 4MB); oldest lines are evicted when exceeded
- On reconnection, buffered content is sent as a `replay` message to the client
- Buffer is automatically reset when the session closes or the process exits

### Idle Timeout

When no WebSocket clients are connected to a session, an idle timer starts (default: 10 minutes). When the timer fires, the PTY process is killed and the session is cleaned up.

### Process Exit

When the PTY process exits (user types `exit`, shell exits, etc.), an `exit` message with the exit code is sent, the PTY file descriptor is cleaned up, the buffer is reset, and the session is removed. The frontend shows a toast notification.

### Process Termination

When a user explicitly closes a session, `SIGTERM` is sent first; if the process doesn't exit within 3 seconds, `SIGKILL` is sent. On Windows, `cmd.Process.Kill()` is used instead.

## Directory Detection & Rebuild

### CWD Resolution Priority

1. `requestedCwd` — explicitly passed working directory parameter
2. `currentFilePath` — directory of the currently viewed file
3. `currentDir` — current directory in the file manager
4. Empty string — defaults to project root

### Directory Change Detection

The terminal does NOT automatically rebuild on directory changes because a long-running command may be active. When the target CWD differs from the session's current CWD, an overlay is shown:

- **"Continue Here"** — dismiss the prompt, keep the current session
- **"Reopen Here"** — kill the current PTY and create a new session in the target directory

### Rebuild Session

Triggered via the 🔄 button in the toolbar or the directory mismatch overlay:

1. Show a "Rebuilding..." spinner overlay
2. Send `POST /api/terminal/close` to kill the PTY process
3. Reset the WebSocket connection, clear errors, clear session ID
4. Clear the xterm.js display
5. Create a new PTY session
6. Hide the spinner overlay

## Android Volume Keys

In the Android App, volume keys are remapped to arrow keys when the terminal is open:

- **Volume Up** → Arrow Up
- **Volume Down** → Arrow Down

Normal volume behavior is restored when the terminal is closed. This is implemented via the Android WebView JS Bridge (`AndroidNative.setVolumeKeyMode`).

## Configuration

All terminal settings are optional — defaults are used when not specified. Configure in `config/config.yaml`:

| Setting | Default | Description |
|---------|---------|-------------|
| `terminal.enabled` | `true` | Enable/disable terminal feature (note: omitting this field in YAML is equivalent to `true`) |
| `terminal.idle_timeout` | `"10m"` | PTY idle timeout when no clients are connected |
| `terminal.buffer_lines` | `2000` | Maximum lines in the ring buffer |
| `terminal.max_line_bytes` | `65536` | Maximum bytes per line (64KB); lines exceeding this are truncated |
| `terminal.max_buffer_mb` | `4` | Total ring buffer memory cap in MB |
| `terminal.max_sessions` | `10` | Maximum concurrent terminal sessions per project |

### Configuration Example

```yaml
terminal:
  enabled: true
  idle_timeout: "10m"
  buffer_lines: 2000
  max_line_bytes: 65536
  max_buffer_mb: 4
  max_sessions: 10
```
