import { describe, expect, test } from "bun:test"

import Kanban from "./kanban"

type Call = { args: readonly string[]; cwd: string }

function harness(options: {
  project?: string
  list?: string
  getSession?: (parameters: any) => Promise<any>
  promptAsync?: (parameters: any) => Promise<any>
} = {}) {
  const calls: Call[] = []
  const prompts: any[] = []
  const dependencies = {
    hooksDisabled: () => false,
    runKb: async (args, cwd) => {
      calls.push({ args, cwd })
      if (args[0] === "detect-project") return options.project ?? "kanban-tui"
      if (args[0] === "list") return options.list ?? "Aug 10  Fix adapter"
      return ""
    },
  }
  const context = {
    directory: "/repo",
    worktree: "/repo-worktree",
    client: {
      session: {
        get: options.getSession ?? (async ({ sessionID }: any) => ({ data: { id: sessionID } })),
        promptAsync: options.promptAsync ?? (async (parameters: any) => {
          prompts.push(parameters)
          return {}
        }),
      },
    },
  }
  return { calls, context, dependencies, prompts }
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
    const h = harness()
    h.dependencies.hooksDisabled = () => true

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

  test("mirrors only the exact memory tool and six accepted types", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "top")

    const rejected = [
      ["engram_mem_save_prompt", { type: "bugfix", title: "prompt" }],
      ["engram_mem_save_extra", { type: "bugfix", title: "extra" }],
      ["engram_mem_save", { type: "preference", title: "preference" }],
      ["engram_mem_save", { type: "unknown", title: "unknown" }],
      ["engram_mem_save", { type: "config", title: "   " }],
    ]
    for (const [tool, args] of rejected) {
      await hooks["tool.execute.after"]({ tool, sessionID: "top", args }, {})
    }
    for (const type of ["bugfix", "decision", "architecture", "discovery", "pattern", "config"]) {
      await hooks["tool.execute.after"]({
        tool: "engram_mem_save",
        sessionID: "top",
        args: { type, title: `saved ${type}` },
      }, {})
    }

    const notes = h.calls.filter((call) => call.args[0] === "note")
    expect(notes).toHaveLength(6)
    expect(notes.map((call) => call.args.at(-1))).toEqual([
      "saved bugfix",
      "saved decision",
      "saved architecture",
      "saved discovery",
      "saved pattern",
      "saved config",
    ])
  })

  test("passes titles, scope, and body as literal argv values", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "top")
    const title = "$(touch /tmp/nope); 'quoted' && false"
    const scope = "scope; rm -rf /"
    const content = "line one\n$(uname)"

    await hooks["tool.execute.after"]({
      tool: "engram_mem_save",
      sessionID: "top",
      args: { type: "decision", title, topic_key: scope, content },
    }, {})

    expect(h.calls.at(-1)?.args).toEqual([
      "note",
      "--project",
      "kanban-tui",
      "--source",
      "hook-post",
      "--scope",
      scope,
      "--body",
      content,
      "--",
      title,
    ])
  })

  test("dispatches one top-level idle audit and prevents recursive loops", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "top")

    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "top" } } })
    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "top" } } })

    expect(h.prompts).toHaveLength(1)
    expect(h.prompts[0].sessionID).toBe("top")
    expect(h.prompts[0].parts[0].text).toContain("Do not run another audit turn")
    expect(h.calls.filter((call) => call.args[0] === "list")).toHaveLength(1)
  })

  test("claims a top-level idle audit before concurrent handlers can dispatch", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "top")
    const idle = { event: { type: "session.idle", properties: { sessionID: "top" } } }

    await Promise.all([hooks.event(idle), hooks.event(idle)])

    expect(h.prompts).toHaveLength(1)
    expect(h.calls.filter((call) => call.args[0] === "list")).toHaveLength(1)
  })

  test("ignores child sessions across system, idle, and tool hooks", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "child", "parent")
    const output = { system: ["original"] }

    await hooks["experimental.chat.system.transform"]({ sessionID: "child" }, output)
    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "child" } } })
    await hooks["tool.execute.after"]({
      tool: "engram_mem_save",
      sessionID: "child",
      args: { type: "bugfix", title: "child fact" },
    }, {})

    expect(output.system).toEqual(["original"])
    expect(h.prompts).toHaveLength(0)
    expect(h.calls.filter((call) => call.args[0] !== "detect-project")).toHaveLength(0)
  })

  test("cleans lifecycle caches when a session is deleted", async () => {
    const h = harness()
    const hooks: any = await plugin(h)
    await created(hooks, "reused")
    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "reused" } } })
    await hooks.event({
      event: { type: "session.deleted", properties: { info: { id: "reused" } } },
    })
    await created(hooks, "reused")
    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "reused" } } })

    expect(h.prompts).toHaveLength(2)
    expect(h.calls.filter((call) => call.args[0] === "list")).toHaveLength(2)
  })

  test("clears the idle marker only when dispatch fails so a later idle retries", async () => {
    let attempts = 0
    const h = harness({
      promptAsync: async () => {
        attempts++
        if (attempts === 1) throw new Error("offline")
        return {}
      },
    })
    const hooks: any = await plugin(h)
    await created(hooks, "top")

    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "top" } } })
    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "top" } } })
    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "top" } } })

    expect(attempts).toBe(2)
  })

  test("preserves existing system content and refreshes durable guidance", async () => {
    const h = harness({ list: "Aug 10  Verify integration" })
    const hooks: any = await plugin(h)
    await created(hooks, "top")
    const output = { system: ["existing system content"] }

    await hooks["experimental.chat.system.transform"]({ sessionID: "top" }, output)
    await hooks["experimental.chat.system.transform"]({ sessionID: "top" }, output)

    expect(output.system[0].startsWith("existing system content")).toBe(true)
    expect(output.system[0]).toContain("Current open tasks for kanban-tui")
    expect(output.system[0]).toContain("this is closed when ___")
    expect(output.system[0]).toContain("completed, confirmed, or observed fact")
    expect(output.system[0]).toContain("avoid duplicates")
    expect(output.system[0]).toContain("Close completed tasks and move active tasks promptly")
    expect(output.system[0]).toContain("Never put local kb task IDs in repository artifacts")
    expect(output.system[0].match(/kb-opencode-guidance:start/g)).toHaveLength(1)
  })

  test("fetches resumed session metadata before deciding top-level status", async () => {
    const fetched: any[] = []
    const h = harness({
      getSession: async (parameters) => {
        fetched.push(parameters)
        return { data: { id: parameters.sessionID } }
      },
    })
    const hooks: any = await plugin(h)

    await hooks.event({ event: { type: "session.idle", properties: { sessionID: "resumed" } } })

    expect(fetched).toHaveLength(1)
    expect(fetched[0].sessionID).toBe("resumed")
    expect(h.prompts).toHaveLength(1)
  })
})
