// Command genchroma writes a static chroma.css combining the dark (dracula)
// and light (github) Chroma styles, scoped via the [data-theme] attribute on
// <html> so a single stylesheet can drive both themes.
//
// The output is a build artifact consumed by the asset fingerprinter; doing
// this at build time replaces a per-request handler with a normal cacheable
// asset.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

func main() {
	out := flag.String("out", "internal/server/static/chroma.css", "output path")
	darkStyle := flag.String("dark", "dracula", "Chroma style name for dark mode")
	lightStyle := flag.String("light", "github", "Chroma style name for light mode")
	flag.Parse()

	dark, err := buildChromaCSS(*darkStyle)
	if err != nil {
		log.Fatalf("build dark css: %v", err)
	}
	light, err := buildChromaCSS(*lightStyle)
	if err != nil {
		log.Fatalf("build light css: %v", err)
	}

	var buf bytes.Buffer
	buf.Write(scopeChromaCSS(dark, `html:not([data-theme="light"]) `))
	buf.Write(scopeChromaCSS(light, `[data-theme="light"] `))

	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	fmt.Fprintf(os.Stderr, "genchroma: wrote %s (%d bytes)\n", *out, buf.Len())
}

func buildChromaCSS(name string) ([]byte, error) {
	style := styles.Get(name)
	if style == nil {
		style = styles.Fallback
	}
	var buf bytes.Buffer
	if err := chromahtml.New(chromahtml.WithClasses(true)).WriteCSS(&buf, style); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// scopeChromaCSS prefixes every selector containing ".chroma" with the given
// scope so dark/light variants can co-exist in one file.
func scopeChromaCSS(css []byte, scope string) []byte {
	var out bytes.Buffer
	for _, line := range bytes.Split(css, []byte("\n")) {
		if idx := bytes.Index(line, []byte(".chroma")); idx >= 0 {
			out.Write(line[:idx])
			out.WriteString(scope)
			out.Write(line[idx:])
		} else {
			out.Write(line)
		}
		out.WriteByte('\n')
	}
	return out.Bytes()
}
