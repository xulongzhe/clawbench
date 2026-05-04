package speech

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// mossNanoCmd is the CLI command for MOSS-TTS-Nano (installed via pip install -e .).
	mossNanoCmd = "moss-tts-nano"
)

// mossNanoDefaultModelDir is the default directory for MOSS-TTS-Nano ONNX model files.
// Package-level var (not const) to allow override in tests.
var mossNanoDefaultModelDir = ".clawbench/moss-nano-models"

// MossNanoProvider implements SpeechProvider using MOSS-TTS-Nano (local, ONNX-based TTS).
//
// MOSS-TTS-Nano is a 0.1B-parameter multilingual speech generation model from MOSI.AI
// and the OpenMOSS team. It supports real-time streaming on CPU via ONNX Runtime,
// produces 48kHz stereo WAV output, and supports ~20 languages including Chinese,
// English, Japanese, Korean, and more.
//
// Installation:
//
//	git clone https://github.com/OpenMOSS/MOSS-TTS-Nano.git
//	cd MOSS-TTS-Nano && pip install -r requirements.txt && pip install -e .
//
// CLI usage:
//
//	moss-tts-nano generate --backend onnx --text "hello" --output out.wav
type MossNanoProvider struct {
	// ModelDir is the directory containing MOSS-TTS-Nano ONNX model files.
	// If empty, models are auto-downloaded by the CLI on first run (to ./models/),
	// or resolved to .clawbench/moss-nano-models/.
	ModelDir string
	// PromptSpeech is the path to a reference audio file for voice cloning.
	// If empty, the model uses a built-in voice preset ("Junhao").
	PromptSpeech string
	// Backend selects the inference backend: "onnx" (default, CPU-friendly) or "pytorch" (requires GPU).
	Backend string
	// Voice is the built-in voice preset name for ONNX backend when no prompt-speech is provided.
	// Default: "Junhao". Only used with ONNX backend.
	Voice string
}

// NewMossNanoProvider creates a MossNanoProvider with sensible defaults.
func NewMossNanoProvider() *MossNanoProvider {
	return &MossNanoProvider{
		Backend: "onnx",
		Voice:   "Junhao",
	}
}

// Synthesize generates an audio file at outputPath using MOSS-TTS-Nano via CLI.
// Text is written to a temporary file and passed via --text-file, since the CLI
// does not support reading from stdin.
func (p *MossNanoProvider) Synthesize(ctx context.Context, text string, outputPath string, _ string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", dir, err)
	}

	// Write text to a temporary file (CLI --text-file does not support stdin "-")
	tmpFile, err := os.CreateTemp("", "moss-nano-text-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // clean up after synthesis
	if _, err := tmpFile.WriteString(text); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Resolve CLI binary path: check .venv/bin/ relative to binary, then $PATH
	cliPath := mossNanoCmd
	if exePath, err := os.Executable(); err == nil {
		candidatePath := filepath.Join(filepath.Dir(exePath), ".venv/bin/moss-tts-nano")
		if _, err := os.Stat(candidatePath); err == nil {
			cliPath = candidatePath
		} else {
			// Fall back to $PATH lookup
			if absPath, err := exec.LookPath(mossNanoCmd); err == nil {
				cliPath = absPath
			}
		}
	}

	args := []string{
		"generate",
		"--backend", p.Backend,
		"--text-file", tmpPath,
		"--output", outputPath,
	}

	// Optional: ONNX model directory (CLI flag: --onnx-model-dir)
	if p.ModelDir != "" {
		args = append(args, "--onnx-model-dir", p.ModelDir)
	}

	// Optional: reference audio for voice cloning (CLI flag: --prompt-speech)
	if p.PromptSpeech != "" {
		args = append(args, "--prompt-speech", p.PromptSpeech)
	}

	// Optional: built-in voice preset for ONNX backend when no prompt-speech
	if p.PromptSpeech == "" && p.Voice != "" {
		args = append(args, "--voice", p.Voice)
	}

	slog.Info("moss-nano synthesize",
		slog.String("output", outputPath),
		slog.String("backend", p.Backend),
		slog.String("model_dir", p.ModelDir),
		slog.String("prompt_speech", p.PromptSpeech),
		slog.String("voice", p.Voice),
		slog.Int("text_len", len(text)),
	)

	cmd := exec.CommandContext(ctx, cliPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("moss-tts-nano failed: %w (stderr: %s)", err, stderr.String())
	}

	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("output file not created: %s", outputPath)
	}

	return nil
}

// ResolveMossNanoModelDir resolves the MOSS-TTS-Nano model directory.
// If modelDir is explicitly set, it is returned as-is.
// Otherwise, it checks the default directory (.clawbench/moss-nano-models);
// if it contains model files (browser_poc_manifest.json exists in a subdirectory),
// the default is returned. Otherwise, returns "" to let the CLI auto-download models.
func ResolveMossNanoModelDir(modelDir string) string {
	if modelDir != "" {
		return modelDir
	}
	// Check if default directory has models (look for browser_poc_manifest.json)
	defaultDir := mossNanoDefaultModelDir
	matches, _ := filepath.Glob(filepath.Join(defaultDir, "*", "browser_poc_manifest.json"))
	if len(matches) > 0 {
		return defaultDir
	}
	return "" // let CLI auto-download
}
