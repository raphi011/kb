package markdown

import (
	"maps"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/wikilink"
)

var mdParser goldmark.Markdown

func init() {
	mdParser = goldmark.New(
		goldmark.WithExtensions(
			extension.TaskList,
			extension.Linkify,
			&wikilink.Extender{},
			meta.Meta,
		),
	)
}

func parseAST(content string) (ast.Node, []byte, map[string]any) {
	source := []byte(content)
	reader := text.NewReader(source)
	pc := parser.NewContext()
	doc := mdParser.Parser().Parse(reader, parser.WithContext(pc))

	var fm map[string]any
	if raw := meta.Get(pc); raw != nil {
		fm = make(map[string]any, len(raw))
		maps.Copy(fm, raw)
	}

	return doc, source, fm
}
