package speech

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// piperCmd is the path to the piper executable, relative to the binary directory.
	piperCmd = ".venv/bin/piper"

	// piperDefaultModelDir is the default directory for Piper model files.
	piperDefaultModelDir = ".clawbench/piper-models"
)

// PiperProvider implements SpeechProvider using Piper (local, offline TTS).
// Piper runs entirely locally — no network required.
type PiperProvider struct {
	// ModelPath is the path to the Piper .onnx model file.
	// If empty, defaults to .clawbench/piper-models/<voice>.onnx.
	ModelPath string
	// NoiseScale controls the randomness of the synthesis (default: 0.667).
	NoiseScale float64
	// LengthScale controls the speech rate (default: 1.0, lower = faster).
	LengthScale float64
	// SentenceSilence is the silence duration between sentences in seconds (default: 0.2).
	SentenceSilence float64
}

// NewPiperProvider creates a PiperProvider with sensible defaults.
func NewPiperProvider() *PiperProvider {
	return &PiperProvider{
		NoiseScale:      0.667,
		LengthScale:     1.0,
		SentenceSilence: 0.2,
	}
}

// Synthesize generates an audio file at outputPath using piper.
// Text is written to a temp file and piped via stdin to avoid shell argument limits.
func (p *PiperProvider) Synthesize(ctx context.Context, text string, outputPath string, _ string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", dir, err)
	}

	// Validate model file exists
	if p.ModelPath == "" {
		return fmt.Errorf("piper model path not configured")
	}
	if _, err := os.Stat(p.ModelPath); err != nil {
		return fmt.Errorf("piper model file not found: %s", p.ModelPath)
	}

	// Resolve piper binary path: check .venv/bin/piper relative to binary, then $PATH
	piperPath := piperCmd
	if exePath, err := os.Executable(); err == nil {
		candidatePath := filepath.Join(filepath.Dir(exePath), piperCmd)
		if _, err := os.Stat(candidatePath); err == nil {
			piperPath = candidatePath
		} else {
			// Fall back to $PATH lookup
			if absPath, err := exec.LookPath("piper"); err == nil {
				piperPath = absPath
			}
		}
	}

	args := []string{
		"--model", p.ModelPath,
		"--output_file", outputPath,
	}

	if p.NoiseScale > 0 {
		args = append(args, "--noise-scale", fmt.Sprintf("%g", p.NoiseScale))
	}
	if p.LengthScale > 0 {
		args = append(args, "--length-scale", fmt.Sprintf("%g", p.LengthScale))
	}
	if p.SentenceSilence > 0 {
		args = append(args, "--sentence-silence", fmt.Sprintf("%g", p.SentenceSilence))
	}

	slog.Info("piper synthesize",
		slog.String("output", outputPath),
		slog.String("model", p.ModelPath),
		slog.Float64("noise_scale", p.NoiseScale),
		slog.Float64("length_scale", p.LengthScale),
		slog.Float64("sentence_silence", p.SentenceSilence),
		slog.Int("text_len", len(text)),
	)

	cmd := exec.CommandContext(ctx, piperPath, args...)
	cmd.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Piper needs shared libraries (libespeak-ng, libonnxruntime, libpiper_phonemize)
	// in its own directory. Set the platform-appropriate library path so the binary can find them.
	if piperDir := filepath.Dir(piperPath); piperDir != "" {
		// piperPath might be a symlink (e.g. .venv/bin/piper -> ../piper/piper)
		// Resolve the symlink to find the actual directory with shared libraries
		if resolved, err := filepath.EvalSymlinks(piperPath); err == nil {
			piperDir = filepath.Dir(resolved)
		}
		// Use the correct library path variable for each platform:
		//   Linux:  LD_LIBRARY_PATH  (.so files)
		//   macOS:  DYLD_LIBRARY_PATH (.dylib files; note: SIP may restrict this for signed binaries)
		//   Windows: PATH (.dll files)
		switch runtime.GOOS {
		case "darwin":
			existing := os.Getenv("DYLD_LIBRARY_PATH")
			if existing == "" {
				cmd.Env = append(os.Environ(), "DYLD_LIBRARY_PATH="+piperDir)
			} else {
				cmd.Env = append(os.Environ(), "DYLD_LIBRARY_PATH="+piperDir+":"+existing)
			}
		case "windows":
			existing := os.Getenv("PATH")
			cmd.Env = append(os.Environ(), "PATH="+piperDir+";"+existing)
		default: // linux and other unix-like systems
			existing := os.Getenv("LD_LIBRARY_PATH")
			if existing == "" {
				cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+piperDir)
			} else {
				cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+piperDir+":"+existing)
			}
		}
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("piper failed: %w (stderr: %s)", err, stderr.String())
	}

	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("output file not created: %s", outputPath)
	}

	return nil
}

// ResolveModelPath resolves the Piper model path from voice name or explicit path.
// If modelPath is explicitly set, it is returned as-is.
// Otherwise, the voice name is used to construct the path: .clawbench/piper-models/<voice>.onnx
func ResolveModelPath(voice, modelPath string) string {
	if modelPath != "" {
		return modelPath
	}
	if voice == "" {
		return ""
	}
	return filepath.Join(piperDefaultModelDir, voice+".onnx")
}
