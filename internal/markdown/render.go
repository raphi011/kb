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
	lookup map[string]string
}

func (r noteResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	target := string(n.Target)
	if r.lookup != nil {
		if path, ok := r.lookup[target]; ok {
			return []byte("/notes/" + path), nil
		}
	}
	return append([]byte("/notes/"), n.Target...), nil
}

// Render converts markdown bytes to HTML with wiki-link resolution, syntax
// highlighting, mermaid support, h1 stripping, and heading collection.
func Render(src []byte, lookup map[string]string) (RenderResult, error) {
	hc := &headingCollector{}
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := newRenderer(lookup, hc).Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return RenderResult{}, fmt.Errorf("render markdown: %w", err)
	}
	return RenderResult{HTML: buf.String(), Headings: hc.headings}, nil
}

func newRenderer(lookup map[string]string, hc *headingCollector) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
			&wikilink.Extender{Resolver: noteResolver{lookup: lookup}},
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&h1Stripper{}, 101),
				util.Prioritized(hc, 102),
				util.Prioritized(&mermaidTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&mermaidNodeRenderer{}, 100),
				util.Prioritized(&externalLinkRenderer{}, 50),
			),
		),
	)
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

// headingCollector extracts h2/h3 headings with their auto-generated IDs.
type headingCollector struct {
	headings []Heading
}

func (hc *headingCollector) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
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
		hc.headings = append(hc.headings, heading)
		return ast.WalkContinue, nil
	})
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
