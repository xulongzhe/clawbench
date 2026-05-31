package cli

import (
	"fmt"
	"net/http"
)

// checkHTTPResponse checks an HTTP response for errors.
// It returns nil if status is http.StatusOK.
// Otherwise it returns an error with the given context.
func checkHTTPResponse(result map[string]any, status int, context string) error {
	if status == http.StatusOK {
		return nil
	}
	errMsg, _ := result["error"].(string)
	if errMsg == "" {
		errMsg = fmt.Sprintf("HTTP %d", status)
	}
	return fmt.Errorf("failed to %s: %s", context, errMsg)
}
