package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileHandler is a slog.Handler that writes JSON logs to a daily-rotated file.
type FileHandler struct {
	opts        slog.HandlerOptions
	dir         string
	prefix      string
	maxDays     int
	mu          sync.Mutex
	file        *os.File
	currentDate string
	group       string // for WithGroup
	attrs       []slog.Attr // for WithAttrs
}

// NewFileHandler creates a file handler that writes to dir/prefix-YYYY-MM-DD.log.
// It rotates daily and removes logs older than maxDays on startup.
func NewFileHandler(dir, prefix string, maxDays int) (*FileHandler, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	h := &FileHandler{
		opts:    slog.HandlerOptions{Level: slog.LevelInfo},
		dir:     dir,
		prefix:  prefix,
		maxDays: maxDays,
	}
	h.cleanup()
	if err := h.rotate(); err != nil {
		return nil, fmt.Errorf("open initial log file: %w", err)
	}
	return h, nil
}

func (h *FileHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *FileHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	today := r.Time.Format("2006-01-02")
	if today != h.currentDate {
		if err := h.rotate(); err != nil {
			return err
		}
	}
	buf := new(strings.Builder)
	slog.NewTextHandler(buf, &h.opts).Handle(context.Background(), r)
	_, err := h.file.WriteString(buf.String() + "\n")
	return err
}

// WithGroup implements slog.Handler
func (h *FileHandler) WithGroup(name string) slog.Handler {
	return &FileHandler{
		opts:    h.opts,
		dir:     h.dir,
		prefix:  h.prefix,
		maxDays: h.maxDays,
		file:    h.file,
		currentDate: h.currentDate,
		group:   h.group + name + ".",
		attrs:   h.attrs,
	}
}

// WithAttrs implements slog.Handler
func (h *FileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs))
	copy(newAttrs, h.attrs)
	return &FileHandler{
		opts:    h.opts,
		dir:     h.dir,
		prefix:  h.prefix,
		maxDays: h.maxDays,
		file:    h.file,
		currentDate: h.currentDate,
		group:   h.group,
		attrs:   append(newAttrs, attrs...),
	}
}

func (h *FileHandler) rotate() error {
	if h.file != nil {
		h.file.Close()
	}
	today := time.Now().Format("2006-01-02")
	name := fmt.Sprintf("%s-%s.log", h.prefix, today)
	path := filepath.Join(h.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	h.file = f
	h.currentDate = today
	return nil
}

func (h *FileHandler) cleanup() {
	pattern := filepath.Join(h.dir, h.prefix+"-*.log")
	matches, _ := filepath.Glob(pattern)
	cutoff := time.Now().AddDate(0, 0, -h.maxDays)
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(path)
		}
	}
}

// Close closes the log file.
func (h *FileHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file != nil {
		return h.file.Close()
	}
	return nil
}

// Write implements io.Writer for use with io.MultiWriter.
func (h *FileHandler) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if today != h.currentDate {
		h.rotate()
	}
	return h.file.Write(p)
}
