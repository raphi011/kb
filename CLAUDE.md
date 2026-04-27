# KB — Personal Knowledge Base

## Route Conventions

- `/api/*` — pure JSON (consumed by JS `fetch`, badges, polling)
- All other routes — HTML / HTMX partials (pages, fragments for `hx-get`/`hx-post`)

## Flashcard Decks

Notes become flashcard decks when tagged with `#flashcards` (inline) or `tags: [flashcards]` in frontmatter. Subtags like `tags: [flashcards/go]` also work.

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

- CSS source: `internal/server/static/css/` (11 layered files, entry: `style.css`)
- JS source: `internal/server/static/js/` (ES modules, entry: `app.js`)
- Bundles: `static/style.min.css`, `static/app.min.js` (esbuild, not committed)
- Build: `just bundle` (or `just bundle-js` / `just bundle-css`)
- Dev: `just dev ~/path/to/repo` (watch mode + server with sourcemaps)
- Docker: bundles both CSS + JS in build stage

## Conventions

Detailed guides for each layer — read before making changes in that area:

- [HTMX patterns](docs/conventions/htmx.md)
- [Templ components](docs/conventions/templ.md)
- [JavaScript](docs/conventions/javascript.md)
- [CSS](docs/conventions/css.md)
- [API routes](docs/conventions/api.md)
