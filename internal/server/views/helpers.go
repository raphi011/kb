package views

import (
	"encoding/json"
	"fmt"
)

func intStr(n int) string {
	return fmt.Sprint(n)
}

// jsonStr returns a JSON-encoded string, safe for embedding in <script> blocks.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
