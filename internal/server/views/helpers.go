package views

import (
	"encoding/json"
	"fmt"
	"strings"
)

func lenStr[T any](s []T) string {
	return fmt.Sprint(len(s))
}

func intStr(n int) string {
	return fmt.Sprint(n)
}

// jsonStr returns a JSON-encoded string, safe for embedding in <script> blocks.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// backlinkDir returns the directory portion of a note path (e.g.
// "work/services/natrium.md" -> "work/services"), or "" for root-level notes.
func backlinkDir(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i]
	}
	return ""
}

