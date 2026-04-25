package markdown

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	wlast "go.abhg.dev/goldmark/wikilink"
)

var tagRegex = regexp.MustCompile(`(?:^|\s)#([a-zA-Z][a-zA-Z0-9_-]*)`)

type MarkdownDoc struct {
	Title         string
	Lead          string
	Body          string
	WordCount     int
	Tags          []string
	WikiLinks     []string
	ExternalLinks []ExternalLink
	Headings      []Heading
	Frontmatter   map[string]any
	Flashcards    []ParsedCard
}

type ExternalLink struct {
	URL   string
	Title string
}

type Heading struct {
	ID    string
	Text  string
	Level int
}

func ParseMarkdown(content string) *MarkdownDoc {
	doc := &MarkdownDoc{
		Frontmatter: make(map[string]any),
	}

	doc.Body = contentAfterFrontmatter(content)
	root, source, _ := parseAST(doc.Body)

	if strings.HasPrefix(content, "---\n") {
		_, _, fm := parseAST(content)
		if fm != nil {
			doc.Frontmatter = fm
		}
	}

	doc.Title = frontmatterString(doc.Frontmatter, "title")
	if doc.Title == "" {
		doc.Title = frontmatterString(doc.Frontmatter, "name")
	}

	w := &astWalker{source: source}
	w.walk(root)

	if doc.Title == "" {
		doc.Title = w.firstH1
	}

	doc.WikiLinks = dedup(w.wikiLinks)
	doc.ExternalLinks = dedupLinks(w.externalLinks)
	doc.Headings = w.headings
	doc.Tags = collectTags(doc.Frontmatter, w.textContent.String())
	doc.Lead = extractLead(doc.Body)
	doc.WordCount = countWords(w.textContent.String())

	if hasFlashcardsTag(doc.Tags) {
		doc.Flashcards = extractFlashcards(doc.Body)
	}

	return doc
}

func contentAfterFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	endIdx := strings.Index(content[4:], "\n---")
	if endIdx <= 0 {
		return content[4:]
	}
	return strings.TrimPrefix(content[4+endIdx+4:], "\n")
}

func collectTags(fm map[string]any, textContent string) []string {
	seen := make(map[string]bool)
	var tags []string

	addTag := func(t string) {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}

	for _, key := range []string{"tags", "labels"} {
		switch v := fm[key].(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					addTag(s)
				}
			}
		case []string:
			for _, s := range v {
				addTag(s)
			}
		}
	}

	for _, match := range tagRegex.FindAllStringSubmatch(textContent, -1) {
		addTag(match[1])
	}

	return tags
}

// hasFlashcardsTag returns true if any tag is "flashcards" or has prefix "flashcards/".
func hasFlashcardsTag(tags []string) bool {
	for _, t := range tags {
		if t == "flashcards" || strings.HasPrefix(t, "flashcards/") {
			return true
		}
	}
	return false
}

func extractLead(body string) string {
	for _, para := range strings.Split(body, "\n\n") {
		trimmed := strings.TrimSpace(para)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "---") {
			continue
		}
		return trimmed
	}
	return ""
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func frontmatterString(fm map[string]any, key string) string {
	if v, ok := fm[key].(string); ok {
		return v
	}
	return ""
}

type astWalker struct {
	source        []byte
	firstH1       string
	wikiLinks     []string
	externalLinks []ExternalLink
	headings      []Heading
	textContent   strings.Builder
}

func (w *astWalker) walk(root ast.Node) {
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		w.visitBlock(node)
	}
}

func (w *astWalker) visitBlock(node ast.Node) {
	switch n := node.(type) {
	case *ast.Heading:
		text := inlineText(w.source, n)
		if w.firstH1 == "" && n.Level == 1 {
			w.firstH1 = text
		}
		if n.Level >= 2 && n.Level <= 3 {
			w.headings = append(w.headings, Heading{Text: text, Level: n.Level})
		}
		w.collectText(n)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		// Skip code blocks for text collection
	default:
		w.collectText(node)
	}
}

func (w *astWalker) collectText(node ast.Node) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			w.textContent.Write(t.Value(w.source))
			w.textContent.WriteByte(' ')
		case *ast.Link:
			dest := string(t.Destination)
			if isExternalURL(dest) {
				w.externalLinks = append(w.externalLinks, ExternalLink{
					URL:   dest,
					Title: inlineText(w.source, t),
				})
			}
		case *ast.AutoLink:
			if t.AutoLinkType == ast.AutoLinkURL {
				url := string(t.URL(w.source))
				if isExternalURL(url) {
					w.externalLinks = append(w.externalLinks, ExternalLink{URL: url})
				}
			}
		case *wlast.Node:
			target := strings.TrimSpace(string(t.Target))
			if target != "" {
				w.wikiLinks = append(w.wikiLinks, target)
			}
		}
		return ast.WalkContinue, nil
	})
}

func inlineText(source []byte, node ast.Node) string {
	var sb strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			sb.Write(t.Value(source))
		} else {
			sb.WriteString(inlineText(source, child))
		}
	}
	return sb.String()
}

func isExternalURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func dedup(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func dedupLinks(links []ExternalLink) []ExternalLink {
	if len(links) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]ExternalLink, 0, len(links))
	for _, l := range links {
		if !seen[l.URL] {
			seen[l.URL] = true
			result = append(result, l)
		}
	}
	return result
}
