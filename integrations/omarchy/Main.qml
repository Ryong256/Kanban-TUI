// Open task count for the Omarchy bar.
//
// The dotfiles installer symlinks this directory to
// ~/.config/omarchy/plugins/io.github.ryong256.kanban, so on the target
// machine the directory name IS the plugin id. Keep it in sync with the
// "id" in manifest.json.
//
// The count is always taken with --all. The bar is one long-lived Quickshell
// process whose working directory has nothing to do with the project you are
// looking at, so kb's cwd-based project auto-detection would resolve against
// that arbitrary directory -- started from $HOME it resolves to no project at
// all. Across every registered project is the only reading that means
// anything from a status bar.

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

BarWidget {
  id: root
  moduleName: "io.github.ryong256.kanban"

  readonly property int refreshIntervalSec: {
    var seconds = parseInt(String(root.setting("refreshIntervalSec", 30)), 10)
    if (!isFinite(seconds)) seconds = 30
    return Math.max(5, Math.min(3600, seconds))
  }

  // Escape hatch for a session whose PATH does not reach kb: `go install`
  // honours GOBIN, so the binary is not always in ~/.local/bin.
  readonly property string kbCommand: String(root.setting("command", "kb"))
  readonly property bool hideWhenEmpty: root.setting("hideWhenEmpty", false) === true
  readonly property string icon: String(root.setting("icon", "󰄹"))

  // -1 until kb answers. "kb never reported" and "kb reported zero" are
  // different states and must not render the same way: an unreachable board
  // is a broken widget, an empty board is good news.
  property int openTasks: -1
  property string detail: ""

  readonly property bool known: root.openTasks >= 0
  readonly property bool empty: root.openTasks === 0
  readonly property bool shown: root.known && !(root.empty && root.hideWhenEmpty)

  function refresh() {
    if (!countProcess.running) countProcess.running = true
  }

  // `kb count --json` prints waybar's shape: text/alt/class/tooltip. Only
  // text and tooltip are read here -- "class" is waybar CSS with no meaning
  // in Quickshell.
  function apply(payload) {
    var raw = String(payload || "").trim()
    if (raw === "") {
      root.clear()
      return
    }

    try {
      var parsed = JSON.parse(raw)
      var count = parseInt(String(parsed.text), 10)
      if (!isFinite(count)) {
        root.clear()
        return
      }
      root.openTasks = count
      root.detail = String(parsed.tooltip || "")
    } catch (error) {
      console.warn(root.moduleName, "ignoring unparseable kb output:", raw)
      root.clear()
    }
  }

  function clear() {
    root.openTasks = -1
    root.detail = ""
  }

  function openBoard() {
    if (!root.bar) return
    // Focuses the board when a window is already up instead of stacking a
    // second terminal on every click.
    root.bar.run("omarchy-launch-or-focus-tui kb-view")
  }

  visible: root.shown
  implicitWidth: root.shown ? indicator.implicitWidth : 0
  implicitHeight: root.shown ? indicator.implicitHeight : 0

  Timer {
    interval: root.refreshIntervalSec * 1000
    repeat: true
    running: true
    triggeredOnStart: true
    onTriggered: root.refresh()
  }

  Process {
    id: countProcess
    command: [root.kbCommand, "count", "--json", "--all"]
    running: false

    // Read the answer off stdout alone. A missing or failing kb writes
    // nothing usable there, and `apply` already treats that as unknown, so
    // there is no need to race the exit code against the draining stream.
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: root.apply(text)
    }
  }

  // The bar shows the icon alone. A single total across every project is a
  // number nobody can act on from a status bar -- it has no notion of the
  // project in front of you, so it only ever reads as a debt counter that
  // never visibly drops. The count belongs on hover, where it is asked for.
  WidgetButton {
    id: indicator
    bar: root.bar
    text: root.icon
    tooltipText: {
      if (root.empty) return "No open tasks"
      var head = root.openTasks + " open tasks"
      return root.detail !== "" ? head + "\n" + root.detail : head
    }
    dimmed: root.empty
    fixedHeight: root.barSize
    onPressed: function(mouseButton) { root.openBoard() }
  }
}
