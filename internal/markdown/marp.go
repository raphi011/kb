package markdown

import (
	"regexp"
	"strings"
)

// SlideInfo holds the number and title of a Marp slide.
type SlideInfo struct {
	Number int
	Title  string
}

var slideHeadingRe = regexp.MustCompile(`^#{1,3}\s+(.+)`)

// extractSlides splits markdown on `---` slide separators and extracts
// a title for each slide (first heading, or first non-empty text line).
func extractSlides(body string) []SlideInfo {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	parts := splitSlides(body)
	slides := make([]SlideInfo, 0, len(parts))

	for i, part := range parts {
		title := slideTitle(part)
		if title == "" {
			title = "(untitled)"
		}
		slides = append(slides, SlideInfo{Number: i + 1, Title: title})
	}

	return slides
}

// splitSlides splits the body on lines that are exactly "---".
func splitSlides(body string) []string {
	var parts []string
	var current strings.Builder

	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "---" {
			content := current.String()
			if strings.TrimSpace(content) != "" || len(parts) > 0 {
				parts = append(parts, content)
			}
			current.Reset()
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}

	if rest := current.String(); strings.TrimSpace(rest) != "" {
		parts = append(parts, rest)
	}

	return parts
}

// slideTitle returns the first heading (h1-h3) in the slide content,
// or the first non-empty, non-comment line as fallback.
func slideTitle(content string) string {
	var fallback string

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "<!--") {
			continue
		}

		if m := slideHeadingRe.FindStringSubmatch(trimmed); m != nil {
			title := strings.ReplaceAll(m[1], "**", "")
			title = strings.ReplaceAll(title, "_", "")
			return strings.TrimSpace(title)
		}

		if fallback == "" && !strings.HasPrefix(trimmed, "-->") && !strings.HasPrefix(trimmed, "<style") {
			fallback = trimmed
		}
	}

	return fallback
}
