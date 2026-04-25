package markdown

import (
	"crypto/sha256"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// FlashcardKind identifies the card format.
type FlashcardKind string

const (
	FlashcardInline    FlashcardKind = "inline"
	FlashcardMultiline FlashcardKind = "multiline"
	FlashcardCloze     FlashcardKind = "cloze"
)

// ParsedCard is the parsed representation of a flashcard extracted from markdown.
type ParsedCard struct {
	Hash     string
	Question string
	Answer   string
	Kind     FlashcardKind
	Reversed bool
	Ord      int
}

// ClozeSpan represents a single cloze deletion within a paragraph.
type ClozeSpan struct {
	ID   string // "c1", "c2", etc.
	Text string // the hidden text
}

// --- Pure-text helpers (testable without goldmark) ---

// extractFlashcards scans the markdown body for all card formats.
func extractFlashcards(body string) []ParsedCard {
	var cards []ParsedCard
	ord := 0

	lines := strings.Split(body, "\n")

	// First pass: inline cards (Q::A and Q:::A)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip headings, code fences, list items
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			continue
		}

		q, a, reversed, ok := splitInlineCard(trimmed)
		if ok {
			hash := cardHash(q, a, FlashcardInline, reversed)
			cards = append(cards, ParsedCard{
				Hash: hash, Question: q, Answer: a,
				Kind: FlashcardInline, Reversed: reversed, Ord: ord,
			})
			ord++
			continue
		}
	}

	// Second pass: multi-line cards (Q\n?\nA or Q\n??\nA)
	cards = append(cards, extractMultilineCards(lines, &ord)...)

	// Third pass: cloze cards from paragraphs
	cards = append(cards, extractClozeCards(body, &ord)...)

	return cards
}

// extractMultilineCards finds Q\n?\nA and Q\n??\nA patterns.
func extractMultilineCards(lines []string, ord *int) []ParsedCard {
	var cards []ParsedCard

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "?" && trimmed != "??" {
			continue
		}
		reversed := trimmed == "??"

		// Collect question lines above
		qLines := collectAbove(lines, i)
		if len(qLines) == 0 {
			continue
		}

		// Collect answer lines below
		aLines := collectBelow(lines, i)
		if len(aLines) == 0 {
			continue
		}

		q := strings.Join(qLines, "\n")
		a := strings.Join(aLines, "\n")
		hash := cardHash(q, a, FlashcardMultiline, reversed)
		cards = append(cards, ParsedCard{
			Hash: hash, Question: q, Answer: a,
			Kind: FlashcardMultiline, Reversed: reversed, Ord: *ord,
		})
		*ord++
	}
	return cards
}

func collectAbove(lines []string, sepIdx int) []string {
	var result []string
	for j := sepIdx - 1; j >= 0; j-- {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" {
			break
		}
		result = append([]string{trimmed}, result...)
	}
	return result
}

func collectBelow(lines []string, sepIdx int) []string {
	var result []string
	for j := sepIdx + 1; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" {
			break
		}
		result = append(result, trimmed)
	}
	return result
}

var (
	clozeHighlightRe = regexp.MustCompile(`==([^=]+)==`)
	clozeAnkiRe      = regexp.MustCompile(`\{\{c(\d+)::([^}]+)\}\}`)
)

// extractClozeCards finds paragraphs with ==x== or {{c1::x}} cloze deletions.
func extractClozeCards(body string, ord *int) []ParsedCard {
	var cards []ParsedCard
	for _, para := range strings.Split(body, "\n\n") {
		trimmed := strings.TrimSpace(para)
		if trimmed == "" {
			continue
		}
		// Skip non-paragraph content
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			continue
		}

		spans := extractClozes(trimmed)
		if len(spans) == 0 {
			continue
		}

		// Each cloze span generates a separate card.
		for _, span := range spans {
			q := trimmed // full paragraph is the question context
			a := span.Text
			hash := cardHash(q+"\x00"+span.ID, a, FlashcardCloze, false)
			cards = append(cards, ParsedCard{
				Hash: hash, Question: q, Answer: a,
				Kind: FlashcardCloze, Reversed: false, Ord: *ord,
			})
			*ord++
		}
	}
	return cards
}

// extractClozes finds all cloze deletions in a paragraph.
func extractClozes(paragraph string) []ClozeSpan {
	var spans []ClozeSpan
	seen := map[string]bool{}

	for _, m := range clozeHighlightRe.FindAllStringSubmatch(paragraph, -1) {
		id := fmt.Sprintf("c%d", len(spans)+1)
		text := m[1]
		key := "hl:" + text
		if !seen[key] {
			seen[key] = true
			spans = append(spans, ClozeSpan{ID: id, Text: text})
		}
	}

	for _, m := range clozeAnkiRe.FindAllStringSubmatch(paragraph, -1) {
		id := "c" + m[1]
		text := m[2]
		key := "anki:" + id
		if !seen[key] {
			seen[key] = true
			spans = append(spans, ClozeSpan{ID: id, Text: text})
		}
	}

	return spans
}

// splitInlineCard splits "Q::A" or "Q:::A" into question, answer, reversed, ok.
func splitInlineCard(s string) (q, a string, reversed bool, ok bool) {
	// Check for reversed first (:::)
	if idx := strings.Index(s, ":::"); idx > 0 && idx < len(s)-3 {
		q = strings.TrimSpace(s[:idx])
		a = strings.TrimSpace(s[idx+3:])
		if q != "" && a != "" {
			return q, a, true, true
		}
	}
	// Check for normal (::)
	if idx := strings.Index(s, "::"); idx > 0 && idx < len(s)-2 {
		// Make sure it's not ::: (already checked above, but confirm the char after idx+2 isn't ':')
		if idx+2 < len(s) && s[idx+2] == ':' {
			return "", "", false, false
		}
		q = strings.TrimSpace(s[:idx])
		a = strings.TrimSpace(s[idx+2:])
		if q != "" && a != "" {
			return q, a, false, true
		}
	}
	return "", "", false, false
}

// splitMultilineCard finds a ? or ?? separator line in paragraph text.
// Returns question, answer, reversed, ok.
func splitMultilineCard(text string) (q, a string, reversed bool, ok bool) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "?" && trimmed != "??" {
			continue
		}
		reversed = trimmed == "??"
		qLines := lines[:i]
		aLines := lines[i+1:]
		q = strings.TrimSpace(strings.Join(qLines, "\n"))
		a = strings.TrimSpace(strings.Join(aLines, "\n"))
		if q != "" && a != "" {
			return q, a, reversed, true
		}
	}
	return "", "", false, false
}

// normalizeWhitespace collapses all runs of whitespace to a single space and trims.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// cardHash computes a stable identity hash for a card.
func cardHash(question, answer string, kind FlashcardKind, reversed bool) string {
	rev := "0"
	if reversed {
		rev = "1"
	}
	data := string(kind) + "\x00" + normalizeWhitespace(question) + "\x00" + normalizeWhitespace(answer) + "\x00" + rev
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h[:8])
}

// --- Goldmark AST transformer + renderer ---

var (
	flashcardKind  = ast.NewNodeKind("Flashcard")
	clozeParaKind  = ast.NewNodeKind("ClozeParagraph")
)

var flashcardsEnabledKey = parser.NewContextKey()

type flashcardNode struct {
	ast.BaseBlock
	question []byte
	answer   []byte
	hash     string
	kind     FlashcardKind
}

func (n *flashcardNode) Kind() ast.NodeKind   { return flashcardKind }
func (n *flashcardNode) IsRaw() bool          { return true }
func (n *flashcardNode) Dump(_ []byte, _ int) {}

// clozeParaNode replaces a paragraph containing cloze deletions.
type clozeParaNode struct {
	ast.BaseBlock
	html []byte // pre-rendered HTML with cloze spans
}

func (n *clozeParaNode) Kind() ast.NodeKind   { return clozeParaKind }
func (n *clozeParaNode) IsRaw() bool          { return true }
func (n *clozeParaNode) Dump(_ []byte, _ int) {}

type flashcardTransformer struct{}

func (t *flashcardTransformer) Transform(doc *ast.Document, reader text.Reader, ctx parser.Context) {
	enabled, ok := ctx.Get(flashcardsEnabledKey).(bool)
	if !ok || !enabled {
		return
	}

	src := reader.Source()

	// First pass: merge sibling paragraphs separated by a ? or ?? paragraph
	// into flashcard nodes (handles blank-line-separated multiline cards).
	t.transformMultilineSiblings(doc, src)

	// Second pass: handle single-paragraph cards (inline, cloze, same-paragraph multiline).
	type replacement struct {
		para *ast.Paragraph
		node ast.Node
	}
	var replacements []replacement

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		p, ok := node.(*ast.Paragraph)
		if !ok {
			return ast.WalkContinue, nil
		}
		text := paragraphText(src, p)

		// Cloze paragraphs take priority (avoids {{c1::x}} matching as inline Q::A).
		if spans := extractClozes(text); len(spans) > 0 {
			rendered := renderClozeHTML(text)
			cn := &clozeParaNode{html: []byte(rendered)}
			replacements = append(replacements, replacement{para: p, node: cn})
			return ast.WalkContinue, nil
		}

		// Multiline Q\n?\nA or Q\n??\nA within one paragraph (no blank lines).
		if q, a, reversed, ok := splitMultilineCard(text); ok {
			hash := cardHash(q, a, FlashcardMultiline, reversed)
			fn := &flashcardNode{
				question: []byte(q),
				answer:   []byte(a),
				hash:     hash,
				kind:     FlashcardMultiline,
			}
			replacements = append(replacements, replacement{para: p, node: fn})
			return ast.WalkContinue, nil
		}

		// Inline Q::A cards.
		if q, a, reversed, ok := splitInlineCard(text); ok {
			hash := cardHash(q, a, FlashcardInline, reversed)
			fn := &flashcardNode{
				question: []byte(q),
				answer:   []byte(a),
				hash:     hash,
				kind:     FlashcardInline,
			}
			replacements = append(replacements, replacement{para: p, node: fn})
		}

		return ast.WalkContinue, nil
	})

	for _, r := range replacements {
		r.para.Parent().ReplaceChild(r.para.Parent(), r.para, r.node)
	}
}

// transformMultilineSiblings finds paragraph sequences [Q-para, ?-para, A-para]
// and replaces all three with a single flashcard node.
func (t *flashcardTransformer) transformMultilineSiblings(doc *ast.Document, src []byte) {
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		// Also descend into container blocks (blockquotes, list items, etc.)
		if node.HasChildren() && node.Kind() != ast.KindParagraph {
			t.transformMultilineSiblingsIn(node, src)
		}
	}
	t.transformMultilineSiblingsIn(doc, src)
}

func (t *flashcardTransformer) transformMultilineSiblingsIn(parent ast.Node, src []byte) {
	node := parent.FirstChild()
	for node != nil {
		sep, ok := node.(*ast.Paragraph)
		if !ok {
			node = node.NextSibling()
			continue
		}
		text := strings.TrimSpace(paragraphText(src, sep))
		if text != "?" && text != "??" {
			node = node.NextSibling()
			continue
		}
		reversed := text == "??"

		prev := prevParagraph(sep)
		next := nextParagraph(sep)
		if prev == nil || next == nil {
			node = node.NextSibling()
			continue
		}

		q := paragraphText(src, prev)
		a := paragraphText(src, next)
		if q == "" || a == "" {
			node = node.NextSibling()
			continue
		}

		hash := cardHash(q, a, FlashcardMultiline, reversed)
		fn := &flashcardNode{
			question: []byte(q),
			answer:   []byte(a),
			hash:     hash,
			kind:     FlashcardMultiline,
		}

		// Save next sibling before removal.
		after := next.NextSibling()

		parent.ReplaceChild(parent, sep, fn)
		parent.RemoveChild(parent, prev)
		parent.RemoveChild(parent, next)

		node = after
	}
}

func prevParagraph(node ast.Node) *ast.Paragraph {
	prev := node.PreviousSibling()
	if p, ok := prev.(*ast.Paragraph); ok {
		return p
	}
	return nil
}

func nextParagraph(node ast.Node) *ast.Paragraph {
	next := node.NextSibling()
	if p, ok := next.(*ast.Paragraph); ok {
		return p
	}
	return nil
}

// renderClozeHTML replaces cloze markers in text with interactive HTML spans.
// Both ==text== and {{c1::text}} are replaced with a clickable reveal element.
func renderClozeHTML(text string) string {
	escaped := html.EscapeString(text)

	// Replace Anki-style {{c1::text}} first (more specific pattern).
	for _, m := range clozeAnkiRe.FindAllStringSubmatch(text, -1) {
		original := html.EscapeString(m[0])
		answer := html.EscapeString(m[2])
		span := fmt.Sprintf(`<span class="cloze" tabindex="0"><span class="cloze-hint">[...]</span><span class="cloze-answer" hidden>%s</span></span>`, answer)
		escaped = strings.Replace(escaped, original, span, 1)
	}

	// Replace highlight-style ==text==.
	for _, m := range clozeHighlightRe.FindAllStringSubmatch(text, -1) {
		original := html.EscapeString(m[0])
		answer := html.EscapeString(m[1])
		span := fmt.Sprintf(`<span class="cloze" tabindex="0"><span class="cloze-hint">[...]</span><span class="cloze-answer" hidden>%s</span></span>`, answer)
		escaped = strings.Replace(escaped, original, span, 1)
	}

	return "<p>" + escaped + "</p>"
}

func paragraphText(src []byte, p *ast.Paragraph) string {
	var buf strings.Builder
	for i := 0; i < p.Lines().Len(); i++ {
		line := p.Lines().At(i)
		buf.Write(line.Value(src))
	}
	return strings.TrimSpace(buf.String())
}

type flashcardNodeRenderer struct{}

func (r *flashcardNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(flashcardKind, r.renderFlashcard)
	reg.Register(clozeParaKind, r.renderCloze)
}

func (r *flashcardNodeRenderer) renderFlashcard(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	fn := node.(*flashcardNode)
	fmt.Fprintf(w, `<div class="flashcard" data-card-hash="%s" data-card-kind="%s">`+"\n", fn.hash, fn.kind)
	fmt.Fprintf(w, `  <div class="flashcard-q">%s</div>`+"\n", fn.question)
	fmt.Fprintf(w, `  <button class="flashcard-reveal" type="button">Show answer</button>`+"\n")
	fmt.Fprintf(w, `  <div class="flashcard-a" hidden>%s</div>`+"\n", fn.answer)
	fmt.Fprintf(w, "</div>\n")
	return ast.WalkContinue, nil
}

func (r *flashcardNodeRenderer) renderCloze(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	cn := node.(*clozeParaNode)
	_, _ = w.Write(cn.html)
	_, _ = w.WriteString("\n")
	return ast.WalkContinue, nil
}
