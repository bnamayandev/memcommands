#!/usr/bin/env bash
# Build memcommands and zelcommands and wire up their shortcuts for the
# current shell on macOS, Linux, or Windows (via WSL or Git Bash):
# memcommands binds Ctrl-R, zelcommands runs whenever you type `zl`.
set -euo pipefail

INSTALL_DIR="$HOME/.local/bin"
MEM_MARKER="# >>> memcommands (Ctrl-R) >>>"
MEM_END_MARKER="# <<< memcommands (Ctrl-R) <<<"
ZEL_MARKER="# >>> zelcommands (zl) >>>"
ZEL_END_MARKER="# <<< zelcommands (zl) <<<"
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
TOTAL_STEPS=6
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

# Detect the operating system. WSL reports "Linux" here (it's a real Linux
# kernel), so it needs no special case; Git Bash/MSYS2 report a MINGW*/MSYS*
# uname, which is native Windows underneath.
case "$(uname -s)" in
  Darwin)       OS="mac" ;;
  Linux)        OS="linux" ;;
  MINGW*|MSYS*) OS="windows" ;;
  *) echo "Unsupported OS: $(uname -s). Only macOS, Linux, and Windows (via WSL or Git Bash) are supported." >&2; exit 1 ;;
esac
echo "Detected OS: $OS"

command -v go >/dev/null 2>&1 || { echo "Go is required to build memcommands and zelcommands. Install Go and retry." >&2; exit 1; }

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

# Build both binaries, naming memcommands so it filters its own invocations.
# (Windows needs the .exe suffix; PATH lookup from bash/PowerShell/cmd all
# append it automatically when you just type the bare name.)
BIN_NAME="memcommands"
ZEL_BIN_NAME="zelcommands"
if [ "$OS" = "windows" ]; then
  BIN_NAME="memcommands.exe"
  ZEL_BIN_NAME="zelcommands.exe"
fi

progress "Building memcommands..."
( cd "$REPO_ROOT" && go build -ldflags="-s -w" -o "bin/$BIN_NAME" ./tui )

progress "Building zelcommands..."
( cd "$REPO_ROOT" && go build -ldflags="-s -w" -o "bin/$ZEL_BIN_NAME" ./zeltui )

mkdir -p "$INSTALL_DIR"
if [ "$OS" = "windows" ]; then
  # Symlinks need admin rights or Developer Mode on Windows, so copy instead;
  # re-run this script after pulling updates to refresh the installed copies.
  cp -f "$REPO_ROOT/bin/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
  progress "Copied $INSTALL_DIR/$BIN_NAME"
  cp -f "$REPO_ROOT/bin/$ZEL_BIN_NAME" "$INSTALL_DIR/$ZEL_BIN_NAME"
  progress "Copied $INSTALL_DIR/$ZEL_BIN_NAME"
else
  # Put them on PATH via symlinks so future rebuilds are picked up automatically.
  ln -sf "$REPO_ROOT/bin/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
  progress "Linked $INSTALL_DIR/$BIN_NAME -> $REPO_ROOT/bin/$BIN_NAME"
  ln -sf "$REPO_ROOT/bin/$ZEL_BIN_NAME" "$INSTALL_DIR/$ZEL_BIN_NAME"
  progress "Linked $INSTALL_DIR/$ZEL_BIN_NAME -> $REPO_ROOT/bin/$ZEL_BIN_NAME"
fi

# Pick the shell and its rc file (macOS bash and Git Bash both launch as login
# shells, which read ~/.bash_profile rather than ~/.bashrc).
SHELL_NAME="$(basename "${SHELL:-}")"
case "$SHELL_NAME" in
  zsh)  RC="$HOME/.zshrc" ;;
  bash) if [ "$OS" = "mac" ] || [ "$OS" = "windows" ]; then RC="$HOME/.bash_profile"; else RC="$HOME/.bashrc"; fi ;;
  fish) RC="$HOME/.config/fish/config.fish"; mkdir -p "$(dirname "$RC")" ;;
  *) printf '\nUnrecognized shell '\''%s'\''. Binaries are installed; add the Ctrl-R binding and a `zl` command manually.\n' "${SHELL_NAME:-unknown}" >&2; exit 0 ;;
esac

touch "$RC"

# Strip a prior snippet (by its marker pair) so a re-run refreshes an outdated
# widget in place, reporting whether it found one to strip.
strip_block() {
  local start="$1" end="$2"
  if ! grep -qF "$start" "$RC"; then
    return 1
  fi
  local tmp
  tmp="$(mktemp)"
  awk -v s="$start" -v e="$end" '
    $0 == s {skip=1}
    !skip {print}
    $0 == e {skip=0}
  ' "$RC" > "$tmp" && mv "$tmp" "$RC"
  return 0
}

# Each binding lives in its own marker pair so either can be refreshed or
# removed independently of the other.
NOTES=()
strip_block "$MEM_MARKER" "$MEM_END_MARKER" && NOTES+=("Ctrl-R binding")
strip_block "$ZEL_MARKER" "$ZEL_END_MARKER" && NOTES+=("zl binding")

case "$SHELL_NAME" in
  zsh)
    cat >> "$RC" <<'EOF'

# >>> memcommands (Ctrl-R) >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
memcommands-widget() { fc -W; memcommands "$BUFFER" </dev/tty; zle reset-prompt }
zle -N memcommands-widget
bindkey '^R' memcommands-widget
# <<< memcommands (Ctrl-R) <<<

# >>> zelcommands (zl) >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
zl() { zelcommands "$@"; }
# <<< zelcommands (zl) <<<
EOF
    ;;
  bash)
    cat >> "$RC" <<'EOF'

# >>> memcommands (Ctrl-R) >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
memcommands-widget() { history -a; memcommands "$READLINE_LINE"; }
bind -x '"\C-r": memcommands-widget'
# <<< memcommands (Ctrl-R) <<<

# >>> zelcommands (zl) >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
zl() { zelcommands "$@"; }
# <<< zelcommands (zl) <<<
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

# >>> zelcommands (zl) >>>
if not contains $HOME/.local/bin $PATH
    set -gx PATH $HOME/.local/bin $PATH
end
function zl
    zelcommands $argv
end
# <<< zelcommands (zl) <<<
EOF
    ;;
esac

progress "Added the Ctrl-R and zl bindings to $RC"

# Success: everything above completed without tripping the ERR trap.
trap - ERR
# Close off the in-place progress bar, then surface any deferred note.
printf '\n'
if [ ${#NOTES[@]} -gt 0 ]; then
  IFS=', '; printf '%sRefreshed existing %s in %s%s\n' "$DIM" "${NOTES[*]}" "$RC" "$RESET"; unset IFS
fi
printf '%s%s✓ Done.%s Open a new %s session (or run: %ssource "%s"%s), then press %sCtrl-R%s for memcommands or type %szl%s for zelcommands.\n' \
  "$BOLD" "$GREEN" "$RESET" "$SHELL_NAME" "$BOLD" "$RC" "$RESET" "$BOLD" "$RESET" "$BOLD" "$RESET"
