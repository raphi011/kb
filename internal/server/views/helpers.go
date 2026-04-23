package views

import (
	"fmt"
	"html"
	"strings"
)

func lenStr[T any](s []T) string {
	return fmt.Sprint(len(s))
}

func intStr(n int) string {
	return fmt.Sprint(n)
}

// backlinkDir returns the directory portion of a note path (e.g.
// "work/services/natrium.md" -> "work/services"), or "" for root-level notes.
func backlinkDir(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i]
	}
	return ""
}

func safeSnippet(s string) string {
	safe := html.EscapeString(s)
	safe = strings.ReplaceAll(safe, "\u27EAMARK_START\u27EB", "<mark>")
	safe = strings.ReplaceAll(safe, "\u27EAMARK_END\u27EB", "</mark>")
	return safe
}
