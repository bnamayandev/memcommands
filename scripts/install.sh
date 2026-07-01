#!/usr/bin/env bash
# Build memcommands and bind Ctrl-R to it for the current shell on macOS or Linux.
set -euo pipefail

INSTALL_DIR="$HOME/.local/bin"
MARKER="# >>> memcommands (Ctrl-R) >>>"
REPO_URL="https://github.com/bnamayandev/memcommands.git"

# Detect the operating system.
case "$(uname -s)" in
  Darwin) OS="mac" ;;
  Linux)  OS="linux" ;;
  *) echo "Unsupported OS: $(uname -s). Only macOS and Linux are supported." >&2; exit 1 ;;
esac
echo "Detected OS: $OS"

command -v go >/dev/null 2>&1 || { echo "Go is required to build memcommands. Install Go and retry." >&2; exit 1; }

# Locate the repo root. When run from a checkout, it's the parent of scripts/;
# when piped from curl there's no checkout, so clone into a cache dir first.
SOURCE="${BASH_SOURCE[0]:-}"
if [ -n "$SOURCE" ] && [ -f "$(cd "$(dirname "$SOURCE")/.." && pwd)/go.mod" ]; then
  REPO_ROOT="$(cd "$(dirname "$SOURCE")/.." && pwd)"
else
  command -v git >/dev/null 2>&1 || { echo "git is required to install from the web. Install git and retry." >&2; exit 1; }
  REPO_ROOT="$HOME/.local/share/memcommands"
  if [ -d "$REPO_ROOT/.git" ]; then
    echo "Updating memcommands source in $REPO_ROOT..."
    git -C "$REPO_ROOT" pull --ff-only
  else
    echo "Cloning memcommands into $REPO_ROOT..."
    mkdir -p "$(dirname "$REPO_ROOT")"
    git clone --depth 1 "$REPO_URL" "$REPO_ROOT"
  fi
fi

# Build the binary, naming it 'memcommands' so it filters its own invocations.
echo "Building memcommands..."
( cd "$REPO_ROOT" && go build -ldflags="-s -w" -o bin/memcommands ./tui )

# Put it on PATH via a symlink so future rebuilds are picked up automatically.
mkdir -p "$INSTALL_DIR"
ln -sf "$REPO_ROOT/bin/memcommands" "$INSTALL_DIR/memcommands"
echo "Linked $INSTALL_DIR/memcommands -> $REPO_ROOT/bin/memcommands"

# Pick the shell and its rc file (macOS bash uses ~/.bash_profile for login shells).
SHELL_NAME="$(basename "${SHELL:-}")"
case "$SHELL_NAME" in
  zsh)  RC="$HOME/.zshrc" ;;
  bash) if [ "$OS" = "mac" ]; then RC="$HOME/.bash_profile"; else RC="$HOME/.bashrc"; fi ;;
  fish) RC="$HOME/.config/fish/config.fish"; mkdir -p "$(dirname "$RC")" ;;
  *) echo "Unrecognized shell '${SHELL_NAME:-unknown}'. Binary is installed; add a Ctrl-R binding manually." >&2; exit 0 ;;
esac

touch "$RC"

# Strip any prior snippet so re-runs refresh an outdated widget in place.
END_MARKER="# <<< memcommands (Ctrl-R) <<<"
if grep -qF "$MARKER" "$RC"; then
  TMP="$(mktemp)"
  awk -v s="$MARKER" -v e="$END_MARKER" '
    $0 == s {skip=1}
    !skip {print}
    $0 == e {skip=0}
  ' "$RC" > "$TMP" && mv "$TMP" "$RC"
  echo "Refreshed existing Ctrl-R binding in $RC"
fi

case "$SHELL_NAME" in
  zsh)
    cat >> "$RC" <<'EOF'

# >>> memcommands (Ctrl-R) >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
memcommands-widget() { fc -W; memcommands "$BUFFER" </dev/tty; zle reset-prompt }
zle -N memcommands-widget
bindkey '^R' memcommands-widget
# <<< memcommands (Ctrl-R) <<<
EOF
    ;;
  bash)
    cat >> "$RC" <<'EOF'

# >>> memcommands (Ctrl-R) >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
memcommands-widget() { history -a; memcommands "$READLINE_LINE"; }
bind -x '"\C-r": memcommands-widget'
# <<< memcommands (Ctrl-R) <<<
EOF
    ;;
  fish)
    cat >> "$RC" <<'EOF'

# >>> memcommands (Ctrl-R) >>>
if not contains $HOME/.local/bin $PATH
    set -gx PATH $HOME/.local/bin $PATH
end
function memcommands-widget
    memcommands (commandline)
    commandline -f repaint
end
bind \cr memcommands-widget
# <<< memcommands (Ctrl-R) <<<
EOF
    ;;
esac

echo "Added Ctrl-R binding to $RC"
echo "Done. Open a new $SHELL_NAME session (or run: source \"$RC\") and press Ctrl-R."
