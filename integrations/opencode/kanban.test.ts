import { describe, expect, test } from "bun:test"
import { chmodSync, mkdtempSync, readFileSync, writeFileSync } from "fs"
import { tmpdir } from "os"
import { join } from "path"

import Kanban from "./kanban"

type Call = { args: readonly string[]; cwd: string; env?: Record<string, string> }

function harness(options: {
  project?: string
  list?: string
  getSession?: (parameters: any) => Promise<any>
  env?: Record<string, string>
  reconcileError?: Error
} = {}) {
  const calls: Call[] = []
  const dependencies = {
    hooksDisabled: () => options.env?.KB_HOOKS_DISABLED === "1",
    reconciling: () => options.env?.KB_RECONCILING === "1",
    runKb: async (args: readonly string[], cwd: string) => {
      const call: Call = { args, cwd }
      calls.push(call)
      if (args[0] === "detect-project") return options.project ?? "kanban-tui"
      if (args[0] === "list") return options.list ?? "Aug 10  Fix adapter"
      if (args[0] === "reconcile") {
        if (options.reconcileError) throw options.reconcileError
        return "{\"schema_version\":1}"
      }
      return ""
    },
  }
  const context = {
    directory: "/repo",
    worktree: "/repo-worktree",
    client: {
      session: {
        get: options.getSession ?? (async ({ sessionID }: any) => ({ data: { id: sessionID } })),
      },
    },
  }
  return { calls, context, dependencies }
}

async function plugin(harness: ReturnType<typeof harness>) {
  return Kanban(harness.context as any, harness.dependencies as any)
}

async function created(hooks: any, id: string, parentID?: string) {
  await hooks.event({
    event: {
      type: "session.created",
      properties: { info: { id, parentID } },
    },
  })
}

describe("OpenCode kanban adapter", () => {
  test("disabled hooks do not invoke kb", async () => {
    const h = harness({ env: { KB_HOOKS_DISABLED: "1" } })
    const hooks = await plugin(h)

    expect(Object.keys(hooks)).toHaveLength(0)
    expect(h.calls).toHaveLength(0)
  })

  test("reconciling marker short-circuits to no hooks", async () => {
    const h = harness({ env: { KB_RECONCILING: "1" } })
    const hooks = await plugin(h)

    expect(Object.keys(hooks)).toHaveLength(0)
    expect(h.calls).toHaveLength(0)
  })

  test("non-project initialization never runs an unscoped list", async () => {
    const h = harness({ project: "" })
    const hooks = await plugin(h)

    expect(Object.keys(hooks)).toHaveLength(0)
    expect(h.calls).toEqual([
      { args: ["detect-project", "/repo-worktree"], cwd: "/repo-worktree" },
    ])
  })

  test("failed project detection is a no-op", async () => {
    const h = harness()
    h.dependencies.runKb = async () => {
      throw new Error("kb unavailable")
    }

    const hooks = await plugin(h)

    expect(Object.keys(hooks)).toHaveLength(0)
  })

  test("invokes reconcile on top-level session idle", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "top")

    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "top" } } })

    const reconciles = h.calls.filter((call) => call.args[0] === "reconcile")
    expect(reconciles).toHaveLength(1)
    expect(reconciles[0].args).toEqual([
      "reconcile",
      "--session-id",
      "top",
      "--json",
    ])
  })

  test("reconcile argv values are literal", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "ses; rm -rf /")

    await hooks.event({
      event: { type: "session.idle", properties: { sessionID: "ses; rm -rf /" } },
    })

    const reconciles = h.calls.filter((call) => call.args[0] === "reconcile")
    expect(reconciles[0].args).toEqual([
      "reconcile",
      "--session-id",
      "ses; rm -rf /",
      "--json",
    ])
  })

  test("failure in reconcile is isolated", async () => {
    const h = harness({ reconcileError: new Error("kb offline") })
    const hooks: any = await plugin(h)
    await created(hooks, "top")

    await expect(hooks.event({ event: { type: "session.idle", properties: { sessionID: "top" } } })).resolves.toBeUndefined()
  })

  test("concurrent idle events dispatch one reconcile", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "top")
    const idle = { event: { type: "session.idle", properties: { sessionID: "top" } } }

    await Promise.all([hooks.event(idle), hooks.event(idle)])

    expect(h.calls.filter((call) => call.args[0] === "reconcile")).toHaveLength(1)
  })

  test("ignores child sessions", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "child", "parent")

    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "child" } } })

    expect(h.calls.filter((call) => call.args[0] === "reconcile")).toHaveLength(0)
  })

  test("production spawn does not mark first reconcile and returns core output", async () => {
    const tmp = mkdtempSync(join(tmpdir(), "kb-spawn-"))
    const log = join(tmp, "calls.log")
    const kb = join(tmp, "kb")
    writeFileSync(
      kb,
      `#!/usr/bin/env bash
echo "$@" >> "${log}"
[[ "$1" == "detect-project" ]] && { echo "proj"; exit; }
[[ "$1" == "reconcile" ]] && { echo '{"schema_version":1,"project":"proj","session_id":"'"$3"'","session_owned":[],"stale_review":[],"errors":[]}'; exit; }
echo "task"
`,
      { mode: 0o755 },
    )

    const oldPath = process.env.PATH
    process.env.PATH = `${tmp}:${oldPath}`
    try {
      const context = {
        directory: tmp,
        worktree: tmp,
        client: { session: { get: async ({ sessionID }: any) => ({ data: { id: sessionID } }) } },
      }
      const hooks: any = await Kanban(context as any, {
        hooksDisabled: () => false,
        reconciling: () => false,
      } as any)
      expect(typeof hooks.event).toBe("function")
      await created(hooks, "top")

      const output = await hooks.event({ event: { type: "session.idle", properties: { sessionID: "top" } } })
      expect(output).toContain('"schema_version":1')
      expect(output).not.toContain('"no_op":true')
      const calls = readFileSync(log, "utf-8").split("\n").filter((line) => line.startsWith("reconcile "))
      expect(calls).toEqual(["reconcile --session-id top --json"])
    } finally {
      process.env.PATH = oldPath
    }
  })
})
