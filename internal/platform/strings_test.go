package platform

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestExtractStringsFromReader_Basic(t *testing.T) {
	// Mix of printable and non-printable bytes
	input := []byte("hello\x00world\x01foo\x00bar\x00baz")
	result := ExtractStringsFromReader(bytes.NewReader(input), 3)

	expected := []string{"hello", "world", "foo", "bar", "baz"}
	if len(result) != len(expected) {
		t.Fatalf("got %d strings, want %d: %v", len(result), len(expected), result)
	}
	for i, s := range expected {
		if result[i] != s {
			t.Errorf("result[%d] = %q, want %q", i, result[i], s)
		}
	}
}

func TestExtractStringsFromReader_MinLength(t *testing.T) {
	input := []byte("ab\x00abc\x00a\x00abcd")
	result := ExtractStringsFromReader(bytes.NewReader(input), 3)

	// "ab" and "a" are too short
	expected := []string{"abc", "abcd"}
	if len(result) != len(expected) {
		t.Fatalf("got %d strings, want %d: %v", len(result), len(expected), result)
	}
	for i, s := range expected {
		if result[i] != s {
			t.Errorf("result[%d] = %q, want %q", i, result[i], s)
		}
	}
}

func TestExtractStringsFromReader_DefaultMinLen(t *testing.T) {
	input := []byte("abc\x00abcde\x00ab")
	// minLen < 1 should default to 4
	result := ExtractStringsFromReader(bytes.NewReader(input), 0)

	// Only "abcde" is >= 4 chars
	if len(result) != 1 || result[0] != "abcde" {
		t.Errorf("got %v, want [abcde]", result)
	}
}

func TestExtractStringsFromReader_EmptyInput(t *testing.T) {
	result := ExtractStringsFromReader(bytes.NewReader(nil), 4)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestExtractStringsFromReader_AllNonPrintable(t *testing.T) {
	input := []byte{0x00, 0x01, 0x02, 0x7F, 0x80, 0xFF}
	result := ExtractStringsFromReader(bytes.NewReader(input), 1)
	if len(result) != 0 {
		t.Errorf("expected empty result for all non-printable, got %v", result)
	}
}

func TestExtractStringsFromReader_TrailingString(t *testing.T) {
	// String at end without trailing non-printable byte
	input := []byte("hello")
	result := ExtractStringsFromReader(bytes.NewReader(input), 3)
	if len(result) != 1 || result[0] != "hello" {
		t.Errorf("got %v, want [hello]", result)
	}
}

func TestExtractStringsFromReader_PrintableRange(t *testing.T) {
	// Test all printable ASCII characters (0x20-0x7E)
	var buf bytes.Buffer
	for b := 0x20; b <= 0x7E; b++ {
		buf.WriteByte(byte(b))
	}
	result := ExtractStringsFromReader(&buf, 1)
	if len(result) != 1 {
		t.Fatalf("expected 1 string, got %d", len(result))
	}
	if len(result[0]) != 95 { // 0x7E - 0x20 + 1 = 95
		t.Errorf("expected 95 chars, got %d", len(result[0]))
	}
}

func TestExtractStringsFromReader_BoundaryBytes(t *testing.T) {
	// 0x1F (non-printable), 0x20 (space, printable), 0x7E (tilde, printable), 0x7F (DEL, non-printable)
	input := []byte{0x1F, 0x20, 0x7E, 0x7F}
	result := ExtractStringsFromReader(bytes.NewReader(input), 1)
	if len(result) != 1 || result[0] != " ~" {
		t.Errorf("got %v, want [ ~]", result)
	}
}

func TestExtractStrings_RealBinary(t *testing.T) {
	// Test against our own test binary
	exe, err := os.Executable()
	if err != nil {
		t.Skip("cannot determine test binary path")
	}

	result, err := ExtractStrings(exe, 4)
	if err != nil {
		t.Fatalf("ExtractStrings failed: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected to find strings in test binary, got none")
	}

	// Should find common strings like "runtime" or "main"
	found := false
	for _, s := range result {
		if strings.Contains(s, "runtime") || strings.Contains(s, "main") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'runtime' or 'main' in test binary strings")
	}
}

func TestExtractStrings_NonExistentFile(t *testing.T) {
	_, err := ExtractStrings("/nonexistent/path/binary", 4)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestExtractStringsFromReader_ReadError(t *testing.T) {
	// Custom reader that returns valid data then a non-EOF error.
	// bufio.ReaderSize is 64KB, so our small payload is buffered in one Read call.
	// The reader must return data with the error on the same call so bufio
	// propagates it when the buffered data is exhausted.
	errReader := &errorAfterReader{
		data: []byte("hello\x00world"),
		err:  io.ErrUnexpectedEOF,
	}

	result := ExtractStringsFromReader(errReader, 3)

	// Both "hello" and "world" are flushed on read error
	if len(result) != 2 || result[0] != "hello" || result[1] != "world" {
		t.Errorf("got %v, want [hello world]", result)
	}
}

// errorAfterReader returns data bytes then err on the next Read call.
type errorAfterReader struct {
	data  []byte
	err   error
	pos   int
	done  bool
}

func (r *errorAfterReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		if !r.done {
			r.done = true
			return 0, r.err
		}
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestExtractStrings_LargeBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip large binary test on Windows (path differences)")
	}

	// Test against the Go binary itself (use the running test executable)
	goPath, err := os.Executable()
	if err != nil {
		t.Skip("cannot determine Go binary path")
	}

	result, err := ExtractStrings(goPath, 4)
	if err != nil {
		t.Fatalf("ExtractStrings failed: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected to find strings in Go binary, got none")
	}
}
