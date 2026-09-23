# Omarchy bar widget

Open task indicator for the [Omarchy](https://omarchy.org/) status bar, read
from `kb count --json --all`. The bar shows the icon alone and reports the
count on hover; clicking the widget opens the board TUI, focusing an existing
window instead of stacking a new terminal on every click.

## Install

Copy this directory into the Omarchy plugin directory, where the directory
name is the plugin id, then place the widget on the bar:

```sh
make install-omarchy
omarchy bar put io.github.ryong256.kanban --section right
```

The target installs a copy, not a symlink: checking out a branch or commit
that predates `integrations/omarchy` would leave a symlink dangling, and
quickshell skips a dangling plugin without any error. Re-run the target after
changing the plugin code here; an earlier symlink install is replaced.

`kb` must be on the session PATH, and `kb-view` must be too for the click
action. The installed plugin under `~/.config/omarchy/plugins/` reloads on save.

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
