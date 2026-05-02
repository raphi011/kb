# KB — Personal Knowledge Base

## Architecture

```
cmd/kb/         CLI (cobra): index, search, list, tags, links, backlinks, cat, edit, sync, serve
internal/
  kb/           Core KB logic: open, index, search, sync, read/write
  index/        SQLite FTS index, incremental/full indexing, link resolution
  gitrepo/      Git operations (go-git): fetch, fast-forward, branch tracking
  markdown/     Goldmark pipeline: frontmatter, wikilinks, syntax highlighting
  server/       HTTP server, HTMX handlers, auth, flashcard review, git smart HTTP
    views/      Templ components (*.templ → *_templ.go via `templ generate`)
    static/     CSS/JS source + bundled output
  srs/          Spaced repetition (FSRS): card scheduling, review state
```

## Commands

```bash
just build          # go build -o kb ./cmd/kb
just test           # go test ./...
just install        # go install + shell completions
just bundle         # esbuild JS + CSS (minified)
just bundle-js      # JS only
just bundle-css     # CSS only
just dev ~/repo     # watch mode + server with sourcemaps
just clean          # rm -f kb
templ generate      # regenerate *_templ.go from *.templ
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `KB_REPO` | Repository path (fallback if not in a git repo) |
| `KB_ORIGIN_URL` | Upstream git URL for sync/serve |
| `KB_ORIGIN_TOKEN` | HTTPS basic auth token for upstream |

## Route Conventions

- `/api/*` — pure JSON (consumed by JS `fetch`, badges, polling)
- All other routes — HTML / HTMX partials (pages, fragments for `hx-get`/`hx-post`)

## Flashcard Decks

Notes become flashcard decks when `flashcards: true` is set in frontmatter (same pattern as `marp: true`).

### Card Formats

**Inline** — single-line Q&A:
```markdown
What is Go::A systems programming language
Berlin:::Capital of Germany          ← reversed (creates card in both directions)
```

**Multi-line** — question/answer separated by `?` or `??`:
```markdown
What is the capital of France
?
Paris

Name the language
??
Go                                   ← reversed
```

**Cloze deletion** — hide parts of a sentence:
```markdown
The capital of France is ==Paris==.
Go was created by {{c1::Google}}.
```
`==text==` (highlight style) and `{{c1::text}}` (Anki style) both work. Each cloze span becomes a separate card.

### Rules

- Headings, code fences, and list items are skipped (not parsed as cards)
- Whitespace-only edits preserve SRS state (card hash is whitespace-normalized)
- Content changes create a new card (old review history is lost)

### Full Example

```markdown
---
tags: [flashcards/go]
---

# Go Basics

What is the zero value of a slice::nil

The `defer` keyword executes a function ==after the surrounding function returns==.

What does `go fmt` do
?
Formats Go source code according to standard style rules
```

### Review

- `/flashcards` — dashboard with stats (new, learning, due today)
- `/flashcards/review` — review due cards; `?note=path/to/note.md` to scope to one note
- Keyboard: Space = show answer / rate Good, 1-4 = rate Again/Hard/Good/Easy, Esc = abort

## Build & Assets

- CSS source: `internal/server/static/css/` (12 layered files, entry: `style.css`)
- JS source: `internal/server/static/js/` (ES modules, entry: `app.js`)
- Bundles: `internal/server/static/style.min.css`, `internal/server/static/app.min.js` (esbuild, gitignored via `*.map`)
- Vendored JS: `htmx.min.js`, `mermaid.min.js`, `marp-*.min.js` (downloaded in Docker build)
- Docker: downloads vendored JS + bundles CSS/JS in build stage

## Conventions

Detailed guides for each layer — read before making changes in that area:

- [HTMX patterns](docs/conventions/htmx.md)
- [Templ components](docs/conventions/templ.md)
- [JavaScript](docs/conventions/javascript.md)
- [CSS](docs/conventions/css.md)
- [API routes](docs/conventions/api.md)
