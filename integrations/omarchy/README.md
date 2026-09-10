# Omarchy bar widget

Open task indicator for the [Omarchy](https://omarchy.org/) status bar, read
from `kb count --json --all`. The bar shows the icon alone and reports the
count on hover; clicking the widget opens the board TUI, focusing an existing
window instead of stacking a new terminal on every click.

## Install

The dotfiles installer symlinks this directory into the Omarchy plugin
directory, so the directory name on the target machine is the plugin id:

```sh
ln -s ~/Projects/Personal/kanban/integrations/omarchy \
      ~/.config/omarchy/plugins/io.github.ryong256.kanban
omarchy bar put io.github.ryong256.kanban --section right
```

`kb` must be on the session PATH, and `kb-view` must be too for the click
action. Plugin code under `~/.config/omarchy/plugins/` reloads on save.

## Settings

Set these on the widget's entry in `~/.config/omarchy/shell.json`, either by
hand or with `omarchy bar set io.github.ryong256.kanban <key> <value>`:

| Key | Default | Meaning |
| --- | --- | --- |
| `refreshIntervalSec` | `30` | Seconds between counts, clamped to 5-3600 |
| `hideWhenEmpty` | `false` | Drop the widget from the bar when nothing is open |
| `command` | `kb` | Path to the kb binary, for a session whose PATH does not reach it |
| `icon` | `󰄹` | Glyph shown in the bar |

## Why `--all`

The bar is one long-lived process whose working directory has nothing to do
with the project you are looking at, so `kb`'s cwd-based project
auto-detection would resolve against that arbitrary directory -- started from
`$HOME` it resolves to no project at all. Counting across every registered
project is the only reading that means anything from a status bar.
