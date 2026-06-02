package platform

import (
	"bufio"
	"io"
	"log/slog"
	"os"
)

// ExtractStrings reads the file at path and extracts printable ASCII strings
// of at least minLen characters. This is a cross-platform replacement for the
// POSIX "strings" command, which does not exist on Windows.
//
// Printable ASCII is defined as bytes in the range 0x20–0x7E (space through tilde).
// If minLen < 1, it defaults to 4.
//
// Note: This function only extracts single-byte ASCII strings. It does not
// handle UTF-16LE strings commonly found in Windows PE resource sections.
// For AI CLI binaries (Claude, Codex), ASCII extraction is sufficient since
// model IDs are always ASCII.
func ExtractStrings(path string, minLen int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ExtractStringsFromReader(f, minLen), nil
}

// ExtractStringsFromReader extracts printable ASCII strings from an io.Reader.
// Strings shorter than minLen are discarded. If minLen < 1, it defaults to 4.
// On non-EOF read errors, partial results collected so far are returned and
// the error is logged.
func ExtractStringsFromReader(r io.Reader, minLen int) []string {
	if minLen < 1 {
		minLen = 4
	}

	var result []string
	buf := make([]byte, 0, 256) // accumulator for current printable run

	br := bufio.NewReaderSize(r, 64*1024)
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			// On read error, flush current run and return partial results
			if len(buf) >= minLen {
				result = append(result, string(buf))
			}
			slog.Debug("extractStrings: read error, returning partial results", "error", err)
			return result
		}

		if b >= 0x20 && b <= 0x7E {
			buf = append(buf, b)
		} else {
			if len(buf) >= minLen {
				result = append(result, string(buf))
			}
			buf = buf[:0]
		}
	}

	// Flush final run
	if len(buf) >= minLen {
		result = append(result, string(buf))
	}

	return result
}
