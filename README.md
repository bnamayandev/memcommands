# memcommands ⌨️

A "keep it super simple" terminal UI for browsing, editing, and re-running your
shell history with Vim keybindings.

Instead of squinting at `Ctrl-R` or scrolling endlessly through `history`,
`vimcommands` drops you into a fuzzy-searchable list of your past commands. Find
the one you want, tweak it in place with the Vim motions you already know, hit
`Enter`, and it runs in your shell.

## ✨ What it does

- 📜 **Reads your real history.** Works with `zsh`, `bash`, and `fish` — it finds
  the right history file automatically (honoring `$HISTFILE`), and falls back to
  asking your shell directly if needed. Entries are normalized and de-duplicated
  so you see each command once, most-recent-first.
- 🔍 **Fuzzy search.** Start typing to filter the list down. Matching is
  alias-aware: searching for a command also turns up its aliases (and the other
  way around).
- 🏷️ **Name your own aliases.** Press `m` on any command to label it, editing
  the label in place with the same Vim motions you use on commands — it behaves
  like part of the line, with only the `[]` fixed. Each command has a single
  alias — pressing `m` again edits the existing one. There's no special save key:
  `Esc` drops back to the command and `:w` writes it, just like editing a
  command; clear the label and save to remove the tag. Aliases are saved to
  `~/.config/memcommands/aliases.json` and become searchable — type the label and
  the command surfaces.
- 🎚️ **Aliased-only view.** Press `Ctrl-a` (in search or the list) to toggle
  between showing everything and showing only commands you've aliased.
- ✏️ **Edit before you run.** Highlight a command and modify it with Vim
  keybindings — change a flag, fix a path, swap an argument — without retyping
  the whole line.
- 🔗 **Alias expansion.** Your shell aliases are loaded and expanded when the
  command is executed, so what runs matches what you'd get in your normal shell.

## 📦 Install

One line — clones the source, builds the binary, and binds `Ctrl-R` for your shell:

```sh
curl -fsSL https://raw.githubusercontent.com/bnamayandev/memcommands/main/scripts/install.sh | bash
```

Open a new shell (or `source` your rc file) and press `Ctrl-R`.

<details>
<summary>Manual build</summary>

```sh
go build -ldflags="-s -w" -o bin/cli ./tui
```

(`-ldflags="-s -w"` strips debug info for a smaller binary.)

Then run the binary:

```sh
./bin/cli
```

(Drop it somewhere on your `$PATH` to call it from anywhere. Or run
`scripts/install.sh` from a checkout to build and wire up `Ctrl-R` in one step.)

</details>

## 🚀 Usage

Launch it and you land in the **search** box. Type to filter your history.

### 🔎 Search

| Key | Action |
| --- | --- |
| _type_ | Fuzzy-filter the history |
| `Enter` | Run the top match |
| `↓` / `↑` / `Ctrl-j` / `Ctrl-n` / `Ctrl-k` / `Ctrl-p` | Drop into the results list |
| `Ctrl-a` | Toggle the aliased-only view |
| `Esc` / `Ctrl-c` | Quit |

### 📋 Results (Normal mode)

Once you're in the list, you're in Vim **normal mode** on the selected command.

| Key | Action |
| --- | --- |
| `j` / `k` / `↓` / `↑` | Move selection down / up (counts work, e.g. `5j`) |
| `Ctrl-d` / `Ctrl-u` | Page down / up |
| `h` / `l` / `←` / `→` | Move the cursor left / right |
| `0` / `$` | Jump to start / end of line |
| `w` / `b` | Next / previous word |
| `e` / `ge` | End of word / end of previous word (`E` `gE` too) |
| `%` | Jump to the matching `(` `[` `{` |
| `i` / `a` / `I` / `A` | Enter insert mode (before/after cursor, line start/end) |
| `x` | Delete character under cursor |
| `d` / `c` / `y` + motion | Delete / change / yank (e.g. `dw`, `d$`, `cc`, `yy`) |
| `v` / `V` | Enter visual mode (`V` selects the whole line) |
| `p` / `P` | Paste after / before cursor |
| `m` | Add an alias, or jump the cursor into its `[brackets]`; the alias is part of the line and edits with the same Vim motions (only the `[]` are fixed). Clear it to remove. |
| `Ctrl-a` | Toggle the aliased-only view |
| `Enter` | Run the (possibly edited) command |
| `Esc` | Back to search |
| `Ctrl-c` | Quit |

### 🔆 Visual mode

Press `v` to start a selection (or `V` to grab the whole line), then extend it
with the usual motions:

| Key | Action |
| --- | --- |
| `h` / `l` / `←` / `→` / `0` / `$` / `w` / `b` / `e` / `ge` / `%` | Extend the selection |
| `y` | Yank the selection to the clipboard |
| `d` / `x` | Delete the selection |
| `c` | Delete the selection and drop into insert mode |
| `Esc` | Cancel back to normal mode |

### ⌨️ Insert mode

Type normally. `←` / `→` move the cursor, `Esc` returns to normal mode, and
`Enter` runs the command.

## ⚙️ How it works

`vimcommands` is a small [Bubble Tea](https://github.com/charmbracelet/bubbletea)
app:

- `core/get_history.go` — locates and parses shell history across zsh/bash/fish.
- `core/aliases.go` — loads shell aliases and expands them at run time.
- `core/user_aliases.go` — persists the aliases you create with `m`.
- `core/fuzzy.go` — the fuzzy matcher behind search.
- `tui/` — the interface: the model, view, and the Vim editor.

Your own invocations of `vimcommands` are filtered out of the list, so it
doesn't clutter its own history.

## 📄 License

See repository.
