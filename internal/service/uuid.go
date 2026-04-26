package service

import (
	"crypto/rand"
	"fmt"
	"log/slog"
)

// generateUUID generates a standard UUID v4 format string with an optional prefix.
// It checks for conflicts in the specified database table and column.
func generateUUID(prefix, tableName, column string) string {
	for i := 0; i < 10; i++ {
		b := make([]byte, 16)
		rand.Read(b)
		// Set version (4) and variant (2) bits according to UUID v4 spec
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		uuid := fmt.Sprintf("%s%x-%x-%x-%x-%x",
			prefix, b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])

		var exists bool
		err := DB.QueryRow(
			fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = ?)", tableName, column),
			uuid,
		).Scan(&exists)
		if err != nil {
			slog.Warn("generateUUID: DB check failed", slog.String("err", err.Error()))
			continue
		}
		if !exists {
			return uuid
		}
	}
	return ""
}
