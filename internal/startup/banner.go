// Package startup renders a structured banner on server ready.
// All content lines are aligned using fmt %-*s with rune-width awareness
// so that CJK characters and emojis do not break the box border.
package startup

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// AgentInfo describes one discovered AI agent for the banner.
type AgentInfo struct {
	Name   string // human-readable agent name (e.g. "codebuddy")
	Models int    // number of available models
}

// BannerConfig holds all data needed to render the startup banner.
type BannerConfig struct {
	Version     string
	Scheme      string // "http" or "https"
	Port        int
	LocalIP     string // non-loopback LAN IP; empty = not available
	AutoPassword string // plaintext auto-generated password; empty = user configured
	DataDir     string // .clawbench/ absolute path
	Agents      []AgentInfo
	SSHEnabled  bool
	SSHPort     int
	TTSEngine   string
	RAGAvailable bool
	TerminalOn  bool
	TaskCount   int
	StartupDuration time.Duration
}

// ---------------------------------------------------------------------------
// rune-width helpers (CJK / emoji aware)
// ---------------------------------------------------------------------------

// runeWidth returns the display width of a string in terminal cells.
// ASCII = 1 cell, CJK ideographs / Hangul / Kana / emoji = 2 cells.
func runeWidth(s string) int {
	w := 0
	for _, r := range s {
		w += charWidth(r)
	}
	return w
}

// charWidth returns the display width of a single rune.
func charWidth(r rune) int {
	// Quick path for ASCII printable + control
	if r < 0x1100 {
		return 1
	}
	// Wide ranges: CJK, Hangul, Kana, emoji, full-width forms, etc.
	switch {
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return 2
	case r == 0x2329 || r == 0x232A: // Angle brackets
		return 2
	case r >= 0x2E80 && r <= 0xA4CF && r != 0x303F: // CJK Radicals..Yi
		return 2
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul Syllables
		return 2
	case r >= 0xF900 && r <= 0xFAFF: // CJK Compatibility Ideographs
		return 2
	case r >= 0xFE10 && r <= 0xFE19: // Vertical forms
		return 2
	case r >= 0xFE30 && r <= 0xFE6F: // CJK Compatibility Forms
		return 2
	case r >= 0xFF01 && r <= 0xFF60: // Fullwidth forms
		return 2
	case r >= 0xFFE0 && r <= 0xFFE6: // Fullwidth signs
		return 2
	case r >= 0x1F300 && r <= 0x1FAFF: // Emoji & symbols
		return 2
	case r >= 0x20000 && r <= 0x2FFFD: // CJK Extension B..I
		return 2
	case r >= 0x30000 && r <= 0x3FFFD: // CJK Extension G..
		return 2
	}
	return 1
}

// padRight pads s with spaces on the right so that its display width equals width.
func padRight(s string, width int) string {
	gap := width - runeWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// ---------------------------------------------------------------------------
// banner line builder
// ---------------------------------------------------------------------------

// buildLines constructs the content lines (without borders).
// Each line is a plain string; the caller pads them to equal rune-width.
func buildLines(cfg BannerConfig) []string {
	var lines []string

	// label formats a label-value pair with emoji and consistent label width.
	// All labels are padded to the same display width so values align vertically.
	labelW := 14 // display width: "🌐 Network:" = 2+1+8+1 = 12, "🔐 Password:" = 2+1+9+1 = 13 → 14
	label := func(prefix, text string) string {
		return padRight(prefix, labelW) + text
	}

	// --- Version ---
	lines = append(lines, fmt.Sprintf("🐾 ClawBench  %s", cfg.Version))
	lines = append(lines, "")

	// --- URLs ---
	localURL := fmt.Sprintf("%s://localhost:%d", cfg.Scheme, cfg.Port)
	lines = append(lines, label("💻 Local:", localURL))
	if cfg.LocalIP != "" {
		networkURL := fmt.Sprintf("%s://%s:%d", cfg.Scheme, cfg.LocalIP, cfg.Port)
		lines = append(lines, label("🌐 Network:", networkURL))
	}

	// --- Password ---
	if cfg.AutoPassword != "" {
		lines = append(lines, label("🔑 Password:", cfg.AutoPassword))
	} else {
		lines = append(lines, label("🔐 Auth:", "password configured"))
	}
	lines = append(lines, "")

	// --- Agents ---
	if len(cfg.Agents) == 0 {
		lines = append(lines, label("🤖 Agents:", "(none — setup wizard will launch)"))
	} else {
		lines = append(lines, "🤖 Agents:")
		// Compute max agent name width for alignment
		maxNameW := 0
		for _, a := range cfg.Agents {
			if rw := runeWidth(a.Name); rw > maxNameW {
				maxNameW = rw
			}
		}
		for _, a := range cfg.Agents {
			modelsStr := ""
			if a.Models == 1 {
				modelsStr = "1 model"
			} else {
				modelsStr = fmt.Sprintf("%d models", a.Models)
			}
			lines = append(lines, fmt.Sprintf("  ● %s  %s", padRight(a.Name, maxNameW), modelsStr))
		}
	}
	lines = append(lines, "")

	// --- SSH (conditional) ---
	if cfg.SSHEnabled {
		sshCmd := fmt.Sprintf("ssh -p %d clawbench@%s", cfg.SSHPort, firstNonEmpty(cfg.LocalIP, "localhost"))
		lines = append(lines, label("🔒 SSH:", sshCmd))
		lines = append(lines, "")
	}

	// --- Data directory ---
	lines = append(lines, label("📁 Data:", cfg.DataDir))
	lines = append(lines, "")

	// --- Service status line ---
	// Note: use single-codepoint emoji only to avoid combining-sequence width issues.
	parts := []string{}
	if cfg.TTSEngine != "" {
		parts = append(parts, fmt.Sprintf("🔊 TTS: %s", cfg.TTSEngine))
	}
	if cfg.RAGAvailable {
		parts = append(parts, "🔍 RAG ✦")
	} else {
		parts = append(parts, "🔍 RAG —")
	}
	if cfg.TerminalOn {
		parts = append(parts, "⌨️ Terminal ✦")
	} else {
		parts = append(parts, "⌨️ Terminal —")
	}
	parts = append(parts, fmt.Sprintf("📋 Tasks: %d", cfg.TaskCount))
	lines = append(lines, strings.Join(parts, "  "))
	lines = append(lines, "")

	// --- Startup duration ---
	lines = append(lines, fmt.Sprintf("⚡ Ready in %s", formatDuration(cfg.StartupDuration)))

	return lines
}

// formatDuration returns a human-friendly duration string.
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// firstNonEmpty returns the first non-empty string argument, or "".
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// PrintBanner renders the startup banner to stdout.
// padRight ensures each line has exactly maxW display cells,
// so we use plain %s — not fmt's %-*s which pads by byte count
// and would misalign CJK/emoji content.
func PrintBanner(cfg BannerConfig) {
	lines := buildLines(cfg)

	// Compute max display width across all lines
	maxW := 0
	for _, l := range lines {
		if rw := runeWidth(l); rw > maxW {
			maxW = rw
		}
	}

	horiz := strings.Repeat("─", maxW)
	fmt.Println()
	fmt.Fprintf(os.Stdout, " ┌%s┐\n", horiz)
	for _, l := range lines {
		fmt.Fprintf(os.Stdout, " │ %s │\n", padRight(l, maxW))
	}
	fmt.Fprintf(os.Stdout, " └%s┘\n", horiz)
	fmt.Println()
}
