package markdown

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

// RenderResult is the output of rendering markdown to HTML.
type RenderResult struct {
	HTML     string
	Headings []Heading
}

// noteResolver resolves [[target]] wiki-links to /notes/<path>.
type noteResolver struct {
	lookup      map[string]string // stem → path
	titleLookup map[string]string // path → title
}

func (r noteResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	target := string(n.Target)
	var dest string
	if r.lookup != nil {
		if path, ok := r.lookup[target]; ok {
			dest = "/notes/" + path
		}
	}
	if dest == "" {
		dest = "/notes/" + target
	}
	if len(n.Fragment) > 0 {
		dest += "#" + string(n.Fragment)
	}
	return []byte(dest), nil
}

// wikilinkRenderer renders [[wiki-links]] with title resolution.
// When no alias is given, it displays the target note's title.
type wikilinkRenderer struct {
	resolver noteResolver
}

func (r *wikilinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(wikilink.Kind, r.render)
}

func (r *wikilinkRenderer) render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*wikilink.Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T", node)
	}

	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}

	dest, err := r.resolver.ResolveWikilink(n)
	if err != nil {
		return ast.WalkStop, err
	}
	if len(dest) == 0 {
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.URLEscape(dest, true))
	_, _ = w.WriteString(`" class="wikilink"`)

	// data-path: the resolved note path (without /notes/ prefix)
	target := string(n.Target)
	notePath := target
	if r.resolver.lookup != nil {
		if path, ok := r.resolver.lookup[target]; ok {
			notePath = path
		}
	}
	_, _ = fmt.Fprintf(w, ` data-path="%s"`, html.EscapeString(notePath))

	if len(n.Fragment) > 0 {
		_, _ = fmt.Fprintf(w, ` data-heading="%s"`, html.EscapeString(string(n.Fragment)))
	}

	_, _ = w.WriteString(`>`)

	// Check if there's an alias: if the child text equals target or target#fragment,
	// no alias was given.
	childText := nodeTextFromWikilink(src, n)
	targetWithFragment := string(n.Target)
	if len(n.Fragment) > 0 {
		targetWithFragment += "#" + string(n.Fragment)
	}
	hasAlias := string(childText) != string(n.Target) && string(childText) != targetWithFragment

	if hasAlias {
		return ast.WalkContinue, nil
	}

	// No alias — resolve title and write it directly.
	path := ""
	if r.resolver.lookup != nil {
		path = r.resolver.lookup[target]
	}
	title := ""
	if path != "" && r.resolver.titleLookup != nil {
		title = r.resolver.titleLookup[path]
	}
	if title != "" {
		_, _ = w.WriteString(html.EscapeString(title))
	} else {
		_, _ = w.Write(util.EscapeHTML(n.Target))
	}
	return ast.WalkSkipChildren, nil
}

func nodeTextFromWikilink(src []byte, n *wikilink.Node) []byte {
	if n.ChildCount() == 0 {
		return nil
	}
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			buf.Write(t.Segment.Value(src))
		}
	}
	return buf.Bytes()
}

// Renderer holds a reusable goldmark.Markdown instance configured with
// wiki-link resolution, syntax highlighting, mermaid, and flashcard support.
// Safe for concurrent use — per-request state flows through parser.Context.
type Renderer struct {
	md          goldmark.Markdown
	lookup      map[string]string
	titleLookup map[string]string
}

// Lookup returns the wiki-link resolution maps used by this renderer.
func (rr *Renderer) Lookup() (lookup map[string]string, titleLookup map[string]string) {
	return rr.lookup, rr.titleLookup
}

// NewRenderer builds a Renderer with the given wiki-link lookup maps.
// The returned instance is safe for concurrent use across requests.
func NewRenderer(lookup map[string]string, titleLookup map[string]string) *Renderer {
	resolver := noteResolver{lookup: lookup, titleLookup: titleLookup}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 199),
			),
			parser.WithASTTransformers(
				util.Prioritized(&h1Stripper{}, 101),
				util.Prioritized(&headingCollector{}, 102),
				util.Prioritized(&mermaidTransformer{}, 100),
				util.Prioritized(&flashcardTransformer{}, 99),
			),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&wikilinkRenderer{resolver: resolver}, 199),
				util.Prioritized(&mermaidNodeRenderer{}, 100),
				util.Prioritized(&flashcardNodeRenderer{lookup: lookup, titleLookup: titleLookup}, 95),
				util.Prioritized(&externalLinkRenderer{}, 50),
			),
		),
	)
	return &Renderer{md: md, lookup: lookup, titleLookup: titleLookup}
}

// Render converts markdown bytes to HTML with wiki-link resolution, syntax
// highlighting, mermaid support, h1 stripping, and heading collection.
func (rr *Renderer) Render(src []byte, flashcardsEnabled bool) (RenderResult, error) {
	var buf bytes.Buffer
	ctx := parser.NewContext()
	ctx.Set(flashcardsEnabledKey, flashcardsEnabled)
	if err := rr.md.Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return RenderResult{}, fmt.Errorf("render markdown: %w", err)
	}
	var headings []Heading
	if h, ok := ctx.Get(headingsKey).([]Heading); ok {
		headings = h
	}
	return RenderResult{HTML: buf.String(), Headings: headings}, nil
}

// Render is a convenience function that creates a one-off renderer.
// Prefer NewRenderer + Render for repeated use with the same lookup maps.
func Render(src []byte, lookup map[string]string, titleLookup map[string]string, flashcardsEnabled bool) (RenderResult, error) {
	return NewRenderer(lookup, titleLookup).Render(src, flashcardsEnabled)
}

// RenderPreview renders markdown for preview popovers. Includes wikilinks,
// GFM, and syntax highlighting but no page-level transforms (h1 stripping,
// heading IDs, mermaid, flashcard transformers).
func RenderPreview(src []byte, lookup map[string]string, titleLookup map[string]string) (RenderResult, error) {
	resolver := noteResolver{lookup: lookup, titleLookup: titleLookup}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 199),
			),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&wikilinkRenderer{resolver: resolver}, 199),
				util.Prioritized(&externalLinkRenderer{}, 50),
			),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return RenderResult{}, fmt.Errorf("render preview: %w", err)
	}
	return RenderResult{HTML: buf.String()}, nil
}

// RenderInline renders a short markdown string (card question/answer) to
// inline HTML. Uses GFM + wikilink parser. No page-level features (h1 stripping,
// heading IDs, mermaid, flashcard transformers). Strips the wrapping <p> tag.
func RenderInline(src string, lookup, titleLookup map[string]string) string {
	resolver := noteResolver{lookup: lookup, titleLookup: titleLookup}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 199),
			),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(&wikilinkRenderer{resolver: resolver}, 199),
			),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return html.EscapeString(src)
	}
	out := strings.TrimSpace(buf.String())
	out = strings.TrimPrefix(out, "<p>")
	out = strings.TrimSuffix(out, "</p>")
	return out
}

// RenderCardQuestion renders a flashcard question. For cloze cards, it
// replaces cloze markers with interactive [...] spans after markdown rendering.
func RenderCardQuestion(question, kind string, lookup, titleLookup map[string]string) string {
	rendered := RenderInline(question, lookup, titleLookup)
	if kind == string(FlashcardCloze) {
		rendered = applyClozeSpans(rendered)
	}
	return rendered
}

// applyClozeSpans replaces cloze markers in rendered HTML with interactive
// reveal spans.
func applyClozeSpans(s string) string {
	s = clozeAnkiRe.ReplaceAllStringFunc(s, func(match string) string {
		m := clozeAnkiRe.FindStringSubmatch(match)
		return fmt.Sprintf(`<span class="cloze" tabindex="0"><span class="cloze-hint">[...]</span><span class="cloze-answer" hidden>%s</span></span>`, html.EscapeString(m[2]))
	})
	s = clozeHighlightRe.ReplaceAllStringFunc(s, func(match string) string {
		m := clozeHighlightRe.FindStringSubmatch(match)
		return fmt.Sprintf(`<span class="cloze" tabindex="0"><span class="cloze-hint">[...]</span><span class="cloze-answer" hidden>%s</span></span>`, html.EscapeString(m[1]))
	})
	return s
}

// h1Stripper removes the first h1 (shown separately in UI header).
type h1Stripper struct{}

func (t *h1Stripper) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := node.(*ast.Heading); ok && h.Level == 1 {
			h.Parent().RemoveChild(h.Parent(), h)
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
}

var headingsKey = parser.NewContextKey()

// headingCollector extracts h2/h3 headings with their auto-generated IDs.
// Headings are stored in the parser.Context (via headingsKey) so the
// goldmark.Markdown instance remains stateless and reusable.
type headingCollector struct{}

func (hc *headingCollector) Transform(doc *ast.Document, reader text.Reader, ctx parser.Context) {
	src := reader.Source()
	var headings []Heading
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := node.(*ast.Heading)
		if !ok || h.Level < 2 || h.Level > 3 {
			return ast.WalkContinue, nil
		}
		var textBuf bytes.Buffer
		for c := h.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				textBuf.Write(t.Segment.Value(src))
			} else {
				_ = ast.Walk(c, func(inner ast.Node, entering bool) (ast.WalkStatus, error) {
					if entering {
						if t, ok := inner.(*ast.Text); ok {
							textBuf.Write(t.Segment.Value(src))
						}
					}
					return ast.WalkContinue, nil
				})
			}
		}
		heading := Heading{Text: textBuf.String(), Level: h.Level}
		if id, ok := h.AttributeString("id"); ok {
			heading.ID = string(id.([]byte))
		}
		headings = append(headings, heading)
		return ast.WalkContinue, nil
	})
	ctx.Set(headingsKey, headings)
}

// --- Mermaid ---

var mermaidKind = ast.NewNodeKind("Mermaid")

type mermaidNode struct {
	ast.BaseBlock
	content []byte
}

func (n *mermaidNode) Kind() ast.NodeKind   { return mermaidKind }
func (n *mermaidNode) IsRaw() bool          { return true }
func (n *mermaidNode) Dump(_ []byte, _ int) {}

type mermaidTransformer struct{}

func (t *mermaidTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var targets []*ast.FencedCodeBlock

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		cb, ok := node.(*ast.FencedCodeBlock)
		if ok && string(cb.Language(src)) == "mermaid" {
			targets = append(targets, cb)
		}
		return ast.WalkContinue, nil
	})

	for _, cb := range targets {
		var buf bytes.Buffer
		for i := 0; i < cb.Lines().Len(); i++ {
			line := cb.Lines().At(i)
			buf.Write(line.Value(src))
		}
		mn := &mermaidNode{content: buf.Bytes()}
		cb.Parent().ReplaceChild(cb.Parent(), cb, mn)
	}
}

type mermaidNodeRenderer struct{}

func (r *mermaidNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(mermaidKind, r.render)
}

func (r *mermaidNodeRenderer) render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	mn := node.(*mermaidNode)
	_, _ = fmt.Fprintf(w, "<pre class=\"mermaid\">%s</pre>\n", html.EscapeString(string(mn.content)))
	return ast.WalkContinue, nil
}

// --- Shared-mode renderers ---

// sharedWikilinkRenderer renders [[wiki-links]] as plain text spans (no link).
// Uses the same alias/title resolution logic as wikilinkRenderer.
type sharedWikilinkRenderer struct {
	resolver noteResolver
}

func (r *sharedWikilinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(wikilink.Kind, r.render)
}

func (r *sharedWikilinkRenderer) render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*wikilink.Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T", node)
	}

	if !entering {
		_, _ = w.WriteString("</span>")
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<span class="wikilink-text">`)

	// Check if there's an alias.
	childText := nodeTextFromWikilink(src, n)
	targetWithFragment := string(n.Target)
	if len(n.Fragment) > 0 {
		targetWithFragment += "#" + string(n.Fragment)
	}
	hasAlias := string(childText) != string(n.Target) && string(childText) != targetWithFragment

	if hasAlias {
		return ast.WalkContinue, nil
	}

	// No alias — resolve title.
	target := string(n.Target)
	path := ""
	if r.resolver.lookup != nil {
		path = r.resolver.lookup[target]
	}
	title := ""
	if path != "" && r.resolver.titleLookup != nil {
		title = r.resolver.titleLookup[path]
	}
	if title != "" {
		_, _ = w.WriteString(html.EscapeString(title))
	} else {
		_, _ = w.Write(util.EscapeHTML(n.Target))
	}
	return ast.WalkSkipChildren, nil
}

// sharedLinkRenderer renders external links normally but strips internal links
// (href starting with "/") to plain text spans.
type sharedLinkRenderer struct{}

func (r *sharedLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
}

func (r *sharedLinkRenderer) renderLink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	dest := string(n.Destination)
	external := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

	if !external {
		// Internal link — render as plain text span.
		if entering {
			_, _ = w.WriteString(`<span>`)
		} else {
			_, _ = w.WriteString(`</span>`)
		}
		return ast.WalkContinue, nil
	}

	if entering {
		_, _ = w.WriteString(`<a href="`)
		_, _ = w.Write(util.EscapeHTML(n.Destination))
		_, _ = w.WriteString(`" target="_blank" rel="noopener"`)
		if n.Title != nil {
			_, _ = w.WriteString(` title="`)
			_, _ = w.Write(util.EscapeHTML(n.Title))
			_, _ = w.WriteString(`"`)
		}
		_, _ = w.WriteString(`>`)
	} else {
		_, _ = w.WriteString(`</a>`)
	}
	return ast.WalkContinue, nil
}

// imageStripper removes *ast.Image nodes from the document tree.
// TODO: support images in shared notes (inline base64 or scoped auth)
type imageStripper struct{}

func (t *imageStripper) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	var images []*ast.Image
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if img, ok := node.(*ast.Image); ok {
			images = append(images, img)
		}
		return ast.WalkContinue, nil
	})
	for _, img := range images {
		parent := img.Parent()
		if parent != nil {
			parent.RemoveChild(parent, img)
		}
	}
}

// RenderShared renders markdown for shared/public note views. Wikilinks become
// plain text, images are stripped, internal links become plain text, external
// links are preserved.
func RenderShared(src []byte, lookup map[string]string, titleLookup map[string]string) (RenderResult, error) {
	resolver := noteResolver{lookup: lookup, titleLookup: titleLookup}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 199),
			),
			parser.WithASTTransformers(
				util.Prioritized(&h1Stripper{}, 101),
				util.Prioritized(&headingCollector{}, 102),
				util.Prioritized(&mermaidTransformer{}, 100),
				util.Prioritized(&imageStripper{}, 98),
			),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&sharedWikilinkRenderer{resolver: resolver}, 199),
				util.Prioritized(&mermaidNodeRenderer{}, 100),
				util.Prioritized(&sharedLinkRenderer{}, 50),
			),
		),
	)
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := md.Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return RenderResult{}, fmt.Errorf("render shared: %w", err)
	}
	var headings []Heading
	if h, ok := ctx.Get(headingsKey).([]Heading); ok {
		headings = h
	}
	return RenderResult{HTML: buf.String(), Headings: headings}, nil
}

// --- External link renderer ---

type externalLinkRenderer struct{}

func (r *externalLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
}

func (r *externalLinkRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	dest := string(n.Destination)
	external := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

	if entering {
		_, _ = w.WriteString(`<a href="`)
		_, _ = w.Write(util.EscapeHTML(n.Destination))
		_, _ = w.WriteString(`"`)
		if external {
			_, _ = w.WriteString(` target="_blank" rel="noopener"`)
		}
		if n.Title != nil {
			_, _ = w.WriteString(` title="`)
			_, _ = w.Write(util.EscapeHTML(n.Title))
			_, _ = w.WriteString(`"`)
		}
		_, _ = w.WriteString(`>`)
	} else {
		_, _ = w.WriteString(`</a>`)
	}
	return ast.WalkContinue, nil
}
