build:
    go build -o kb ./cmd/kb

install: && install-completions
    go install ./cmd/kb

install-completions:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -z "${HOME:-}" ]]; then
        echo "Warning: \$HOME is not set, skipping completions" >&2
        exit 0
    fi
    gobin="$(go env GOBIN)"
    kb_bin="${gobin:-$(go env GOPATH)/bin}/kb"
    shell_name="$(basename "${SHELL:-}")"
    case "$shell_name" in
        fish)
            dest="$HOME/.config/fish/completions/kb.fish"
            ;;
        bash)
            dest="$HOME/.local/share/bash-completion/completions/kb"
            ;;
        zsh)
            brew_prefix="$(HOMEBREW_NO_AUTO_UPDATE=1 brew --prefix 2>/dev/null)" || brew_prefix=""
            if [[ -n "$brew_prefix" ]]; then
                dest="$brew_prefix/share/zsh/site-functions/_kb"
            else
                dest="$HOME/.zfunc/_kb"
            fi
            ;;
        *)
            if [[ -z "$shell_name" ]]; then
                echo "Warning: \$SHELL is not set, skipping completions" >&2
            else
                echo "Warning: unsupported shell '$shell_name', skipping completions" >&2
            fi
            echo "Run manually: kb completion <shell>" >&2
            exit 0
            ;;
    esac
    mkdir -p "$(dirname "$dest")"
    tmpfile="$(mktemp)"
    trap 'rm -f "$tmpfile"' EXIT
    "$kb_bin" completion "$shell_name" > "$tmpfile"
    if [[ ! -s "$tmpfile" ]]; then
        echo "Error: completion generation produced empty output" >&2
        exit 1
    fi
    mv "$tmpfile" "$dest"
    echo "Installed completions to $dest"

test:
    go test ./...

clean:
    rm -f kb

bundle-js:
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js

bundle-css:
    npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css

gen-chroma:
    go run ./cmd/genchroma -out internal/server/static/chroma.css

# Fingerprint every asset referenced by HTML so it can be served immutably.
# Add a logical name to the -files list whenever a new asset is introduced
# (and a corresponding Asset("…") call in the templates).
gen-assets:
    go run ./cmd/genassets \
        -dir internal/server/static \
        -out internal/server/static/dist \
        -manifest internal/server/static/dist/manifest.json \
        -files app.min.js,style.min.css,chroma.css,htmx.min.js,mermaid.min.js,marp-core.min.js,marp-browser.min.js

bundle: bundle-js bundle-css gen-chroma gen-assets

dev repo:
    #!/usr/bin/env bash
    set -euo pipefail
    # chroma.css is built once: changes only when the chroma library or theme
    # selection changes, neither of which happens during a dev session.
    go run ./cmd/genchroma -out internal/server/static/chroma.css
    npx esbuild internal/server/static/css/style.css --bundle --sourcemap --outfile=internal/server/static/style.min.css --watch &
    npx esbuild internal/server/static/js/app.js --bundle --sourcemap --format=iife --outfile=internal/server/static/app.min.js --watch &
    go run ./cmd/kb serve --token test --repo "{{ repo }}"
