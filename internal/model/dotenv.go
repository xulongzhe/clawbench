package model

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv reads a .env file and sets each KEY=VALUE pair into the
// current process environment via os.Setenv. Lines starting with '#' and
// blank lines are skipped. Values may be unquoted, double-quoted, or
// single-quoted. Existing environment variables are overwritten.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("dotenv: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}

		key, value, err := parseEnvLine(trimmed)
		if err != nil {
			return fmt.Errorf("dotenv: line %d: %w", lineNo, err)
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}

// parseEnvLine parses a single KEY=VALUE line.
// VALUE may be unquoted, double-quoted ("value"), or single-quoted ('value').
// Quoted values may contain spaces; unquoted values are trimmed at the first
// whitespace or # comment.
func parseEnvLine(line string) (string, string, error) {
	idx := strings.Index(line, "=")
	if idx < 1 { // key must be non-empty
		return "", "", fmt.Errorf("invalid format: %q", line)
	}
	key := strings.TrimSpace(line[:idx])
	if key == "" {
		return "", "", fmt.Errorf("empty key: %q", line)
	}

	raw := line[idx+1:]

	// Quoted values
	if len(raw) >= 2 {
		switch {
		case raw[0] == '"' && raw[len(raw)-1] == '"':
			return key, raw[1 : len(raw)-1], nil
		case raw[0] == '\'' && raw[len(raw)-1] == '\'':
			return key, raw[1 : len(raw)-1], nil
		}
	}

	// Unquoted: trim trailing inline comment (space + #)
	if i := strings.Index(raw, " #"); i >= 0 {
		raw = raw[:i]
	}
	return key, strings.TrimSpace(raw), nil
}
