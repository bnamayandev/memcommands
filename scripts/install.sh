#!/usr/bin/env bash
# Build memcommands and bind Ctrl-R to it for the current shell on macOS or Linux.
set -euo pipefail

INSTALL_DIR="$HOME/.local/bin"
MARKER="# >>> memcommands (Ctrl-R) >>>"
REPO_URL="https://github.com/bnamayandev/memcommands.git"

# Colors, but only when writing to a real terminal and NO_COLOR is unset.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; GREEN=$'\033[32m'; RED=$'\033[31m'; RESET=$'\033[0m'
  TTY=1
else
  BOLD=''; DIM=''; GREEN=''; RED=''; RESET=''
  TTY=0
fi

# Step-based progress bar. Redraws on a single line via \r so it fills in place
# rather than stacking; the bar is neutral (green is reserved for success).
STEP=0
TOTAL_STEPS=4
progress() {
  STEP=$((STEP + 1))
  local width=24 filled empty i bar=''
  filled=$((STEP * width / TOTAL_STEPS))
  empty=$((width - filled))
  for ((i = 0; i < filled; i++)); do bar+='#'; done
  for ((i = 0; i < empty; i++)); do bar+='-'; done
  if [ "$TTY" = 1 ]; then
    # \r returns to column 0; \033[K clears the rest of the old line.
    printf '\r%s[%s] %d/%d  %s%s\033[K' "$DIM" "$bar" "$STEP" "$TOTAL_STEPS" "$1" "$RESET"
  else
    printf '[%s] %d/%d  %s\n' "$bar" "$STEP" "$TOTAL_STEPS" "$1"
  fi
}

# On any unexpected failure, report it in red before exiting. The leading \n
# breaks off the in-place progress bar so the message lands on its own line.
fail() { printf '\n%s%s✗ Install failed.%s %s\n' "$BOLD" "$RED" "$RESET" "${1:-See the error above.}" >&2; }
trap 'fail' ERR

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
  progress "Using local checkout at $REPO_ROOT..."
else
  command -v git >/dev/null 2>&1 || { echo "git is required to install from the web. Install git and retry." >&2; exit 1; }
  REPO_ROOT="$HOME/.local/share/memcommands"
  if [ -d "$REPO_ROOT/.git" ]; then
    progress "Updating memcommands source in $REPO_ROOT..."
    git -C "$REPO_ROOT" pull --ff-only --quiet
  else
    progress "Cloning memcommands into $REPO_ROOT..."
    mkdir -p "$(dirname "$REPO_ROOT")"
    git clone --depth 1 --quiet "$REPO_URL" "$REPO_ROOT"
  fi
fi

# Build the binary, naming it 'memcommands' so it filters its own invocations.
progress "Building memcommands..."
( cd "$REPO_ROOT" && go build -ldflags="-s -w" -o bin/memcommands ./tui )

# Put it on PATH via a symlink so future rebuilds are picked up automatically.
mkdir -p "$INSTALL_DIR"
ln -sf "$REPO_ROOT/bin/memcommands" "$INSTALL_DIR/memcommands"
progress "Linked $INSTALL_DIR/memcommands -> $REPO_ROOT/bin/memcommands"

# Pick the shell and its rc file (macOS bash uses ~/.bash_profile for login shells).
SHELL_NAME="$(basename "${SHELL:-}")"
case "$SHELL_NAME" in
  zsh)  RC="$HOME/.zshrc" ;;
  bash) if [ "$OS" = "mac" ]; then RC="$HOME/.bash_profile"; else RC="$HOME/.bashrc"; fi ;;
  fish) RC="$HOME/.config/fish/config.fish"; mkdir -p "$(dirname "$RC")" ;;
  *) printf '\nUnrecognized shell '\''%s'\''. Binary is installed; add a Ctrl-R binding manually.\n' "${SHELL_NAME:-unknown}" >&2; exit 0 ;;
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
  # Defer this note so it doesn't interrupt the in-place progress bar.
  NOTE="Refreshed an existing Ctrl-R binding in $RC"
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

progress "Added Ctrl-R binding to $RC"

# Success: everything above completed without tripping the ERR trap.
trap - ERR
# Close off the in-place progress bar, then surface any deferred note.
printf '\n'
[ -n "${NOTE:-}" ] && printf '%s%s%s\n' "$DIM" "$NOTE" "$RESET"
printf '%s%s✓ Done.%s Open a new %s session (or run: %ssource "%s"%s) and press %sCtrl-R%s.\n' \
  "$BOLD" "$GREEN" "$RESET" "$SHELL_NAME" "$BOLD" "$RC" "$RESET" "$BOLD" "$RESET"
