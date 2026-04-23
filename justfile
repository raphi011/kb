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
