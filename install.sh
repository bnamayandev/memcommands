#!/usr/bin/env bash
# Build memcommands and bind Ctrl-R to it for the current shell on macOS or Linux.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$HOME/.local/bin"
MARKER="# >>> memcommands (Ctrl-R) >>>"

# Detect the operating system.
case "$(uname -s)" in
  Darwin) OS="mac" ;;
  Linux)  OS="linux" ;;
  *) echo "Unsupported OS: $(uname -s). Only macOS and Linux are supported." >&2; exit 1 ;;
esac
echo "Detected OS: $OS"

# Build the binary, naming it 'memcommands' so it filters its own invocations.
command -v go >/dev/null 2>&1 || { echo "Go is required to build memcommands. Install Go and retry." >&2; exit 1; }
echo "Building memcommands..."
( cd "$SCRIPT_DIR" && go build -ldflags="-s -w" -o bin/memcommands ./tui )

# Put it on PATH via a symlink so future rebuilds are picked up automatically.
mkdir -p "$INSTALL_DIR"
ln -sf "$SCRIPT_DIR/bin/memcommands" "$INSTALL_DIR/memcommands"
echo "Linked $INSTALL_DIR/memcommands -> $SCRIPT_DIR/bin/memcommands"

# Pick the shell and its rc file (macOS bash uses ~/.bash_profile for login shells).
SHELL_NAME="$(basename "${SHELL:-}")"
case "$SHELL_NAME" in
  zsh)  RC="$HOME/.zshrc" ;;
  bash) if [ "$OS" = "mac" ]; then RC="$HOME/.bash_profile"; else RC="$HOME/.bashrc"; fi ;;
  fish) RC="$HOME/.config/fish/config.fish"; mkdir -p "$(dirname "$RC")" ;;
  *) echo "Unrecognized shell '${SHELL_NAME:-unknown}'. Binary is installed; add a Ctrl-R binding manually." >&2; exit 0 ;;
esac

touch "$RC"

# Append the Ctrl-R snippet once; re-runs are no-ops.
if grep -qF "$MARKER" "$RC"; then
  echo "Ctrl-R binding already present in $RC"
  echo "Done. Open a new $SHELL_NAME session (or run: source \"$RC\") and press Ctrl-R."
  exit 0
fi

case "$SHELL_NAME" in
  zsh)
    cat >> "$RC" <<'EOF'

# >>> memcommands (Ctrl-R) >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
memcommands-widget() { memcommands </dev/tty; zle reset-prompt }
zle -N memcommands-widget
bindkey '^R' memcommands-widget
# <<< memcommands (Ctrl-R) <<<
EOF
    ;;
  bash)
    cat >> "$RC" <<'EOF'

# >>> memcommands (Ctrl-R) >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
bind -x '"\C-r": memcommands'
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
    memcommands
    commandline -f repaint
end
bind \cr memcommands-widget
# <<< memcommands (Ctrl-R) <<<
EOF
    ;;
esac

echo "Added Ctrl-R binding to $RC"
echo "Done. Open a new $SHELL_NAME session (or run: source \"$RC\") and press Ctrl-R."
