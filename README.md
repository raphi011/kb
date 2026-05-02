# kb

Git-backed markdown knowledge base with full-text search, wikilinks, flashcards, and a web UI.

## Features

- **Full-text search** — SQLite FTS index with incremental updates
- **Wikilinks** — `[[link]]` resolution with backlink tracking
- **Flashcards** — Spaced repetition (FSRS) via frontmatter-enabled notes
- **Web UI** — HTMX-powered interface with file tree, calendar, TOC, and syntax highlighting
- **Git sync** — Pull/push to a remote; serves git smart HTTP for cloning
- **CLI** — Index, search, list, tags, links, backlinks, cat, edit, sync, serve

## Quick Start

```bash
go install github.com/raphi011/kb/cmd/kb@latest

# Point at a git repo of markdown files
export KB_REPO=~/notes

# Build the index
kb index

# Start the web server
kb serve
```

## Build

```bash
just build       # go build -o kb ./cmd/kb
just test        # go test ./...
just bundle      # esbuild JS + CSS
just dev ~/repo  # watch mode + dev server
```

## Environment

| Variable | Purpose |
|----------|---------|
| `KB_REPO` | Path to markdown repository |
| `KB_ORIGIN_URL` | Upstream git URL for sync |
| `KB_ORIGIN_TOKEN` | HTTPS auth token for upstream |
