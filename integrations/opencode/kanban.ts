import type { Plugin } from "@opencode-ai/plugin"

const MEMORY_TOOL = "engram_mem_save"
const GUIDANCE_START = "<!-- kb-opencode-guidance:start -->"
const GUIDANCE_END = "<!-- kb-opencode-guidance:end -->"
const MIRRORED_TYPES = new Set([
  "bugfix",
  "decision",
  "architecture",
  "discovery",
  "pattern",
  "config",
])

const OPERATING_PROCEDURE = `## kb task and note procedure

- A task completes only when its checkable condition is true. State that condition as: \`this is closed when ___\`.
- A note is a completed, confirmed, or observed fact. Notes do not have a lifecycle.
- Search the current project tasks before adding work and avoid duplicates.
- Close completed tasks and move active tasks promptly so the board reflects reality.
- Never put local kb task IDs in repository artifacts, including source, tests, documentation, commits, issues, or pull requests.`

const AUDIT_PROMPT = `Run one concise kb audit for this session now. Review the current project tasks and the work just performed. Add only genuinely missing future work, using \`this is closed when ___\` as the completion condition. Record completed, confirmed, or observed facts as notes instead of tasks. Avoid duplicates, and close or move existing tasks promptly. Never put local kb task IDs in repository artifacts. Do not run another audit turn in response to this message.`

type SessionInfo = {
  id?: string
  parentID?: string
}

type ApiResult<T> = {
  data?: T
  error?: unknown
}

type SessionClient = {
  get: (parameters: unknown) => Promise<ApiResult<SessionInfo> | SessionInfo | undefined>
  promptAsync: (parameters: unknown) => Promise<ApiResult<void> | undefined>
}

type PluginContext = {
  client: { session: SessionClient }
  directory: string
  worktree?: string
}

type KanbanDependencies = {
  hooksDisabled: () => boolean
  runKb: (args: readonly string[], cwd: string) => Promise<string | undefined>
}

async function runKb(args: readonly string[], cwd: string): Promise<string | undefined> {
  try {
    const bun = (globalThis as any).Bun
    const child = bun.spawn(["kb", ...args], {
      cwd,
      stdin: "ignore",
      stdout: "pipe",
      stderr: "ignore",
    })
    const stdout = await new Response(child.stdout).text()
    if ((await child.exited) !== 0) return undefined
    return stdout.trim()
  } catch {
    return undefined
  }
}

const defaultDependencies: KanbanDependencies = {
  hooksDisabled: () => (globalThis as any).process?.env?.KB_HOOKS_DISABLED === "1",
  runKb,
}

function dataFrom<T>(result: ApiResult<T> | T | undefined): T | undefined {
  if (!result || typeof result !== "object") return undefined
  if ("error" in result && result.error) return undefined
  if ("data" in result) return result.data
  return result as T
}

function failed(result: ApiResult<unknown> | undefined): boolean {
  return !!result && typeof result === "object" && "error" in result && !!result.error
}

function removeExistingGuidance(system: string[]): void {
  const block = new RegExp(`\\n?${GUIDANCE_START}[\\s\\S]*?${GUIDANCE_END}`, "g")
  for (let index = 0; index < system.length; index++) {
    system[index] = system[index].replace(block, "")
  }
}

function appendGuidance(system: string[], guidance: string): void {
  removeExistingGuidance(system)
  if (system.length === 0) {
    system.push(guidance)
    return
  }
  system[system.length - 1] += `\n\n${guidance}`
}

const Kanban = (async (rawContext, rawOptions) => {
  const overrides = (rawOptions ?? {}) as Partial<KanbanDependencies>
  const dependencies = { ...defaultDependencies, ...overrides }
  try {
    if (dependencies.hooksDisabled()) return {}

    const context = rawContext as unknown as PluginContext
    const cwd = context.worktree || context.directory
    const project = (await dependencies.runKb(["detect-project", cwd], cwd))?.trim()
    if (!project) return {}

    const sessionKinds = new Map<string, "top-level" | "child">()
    const tasksBySession = new Map<string, string>()
    const auditDispatched = new Set<string>()

    const rememberSession = (info: SessionInfo | undefined): boolean => {
      if (!info?.id) return false
      const kind = info.parentID ? "child" : "top-level"
      sessionKinds.set(info.id, kind)
      return kind === "top-level"
    }

    const isTopLevel = async (sessionID: string): Promise<boolean> => {
      const known = sessionKinds.get(sessionID)
      if (known) return known === "top-level"

      try {
        // OpenCode 1.18.16's current client uses sessionID. Its PluginInput
        // declaration still exposes legacy path/body fields, so both aliases
        // are supplied to the same request until those published types converge.
        const result = await context.client.session.get({
          sessionID,
          directory: cwd,
          path: { id: sessionID },
          query: { directory: cwd },
        })
        const info = dataFrom(result)
        return rememberSession(info)
      } catch {
        return false
      }
    }

    const refreshTasks = async (sessionID: string): Promise<boolean> => {
      const tasks = await dependencies.runKb(["list", "--project", project], cwd)
      if (tasks === undefined) return false
      tasksBySession.set(sessionID, tasks || "no open tasks")
      return true
    }

    const hooks = {
      event: async ({ event }: { event: any }) => {
        try {
          if (event.type === "session.created") {
            rememberSession(event.properties?.info)
            return
          }

          if (event.type === "session.deleted") {
            const sessionID = event.properties?.info?.id
            if (!sessionID) return
            sessionKinds.delete(sessionID)
            tasksBySession.delete(sessionID)
            auditDispatched.delete(sessionID)
            return
          }

          if (event.type !== "session.idle") return
          const sessionID = event.properties?.sessionID
          if (!sessionID || auditDispatched.has(sessionID)) return
          if (!(await isTopLevel(sessionID))) return
          if (auditDispatched.has(sessionID)) return
          auditDispatched.add(sessionID)
          try {
            if (!(await refreshTasks(sessionID))) throw new Error("kb task refresh failed")
            const parts = [{ type: "text", text: AUDIT_PROMPT }]
            const result = await context.client.session.promptAsync({
              sessionID,
              directory: cwd,
              parts,
              path: { id: sessionID },
              query: { directory: cwd },
              body: { parts },
            })
            if (failed(result)) throw new Error("OpenCode rejected the kb audit prompt")
          } catch {
            auditDispatched.delete(sessionID)
          }
        } catch {
          // Kanban automation is advisory and must never break OpenCode.
        }
      },

      "experimental.chat.system.transform": async (
        input: { sessionID?: string },
        output: { system: string[] },
      ) => {
        try {
          const sessionID = input.sessionID
          if (!sessionID || !(await isTopLevel(sessionID))) return
          const refreshed = await refreshTasks(sessionID)
          const tasks = tasksBySession.get(sessionID)
          if (!refreshed && !tasks) return

          const guidance = `${GUIDANCE_START}\n${OPERATING_PROCEDURE}\n\n### Current open tasks for ${project}\n\n${tasks}\n${GUIDANCE_END}`
          appendGuidance(output.system, guidance)
        } catch {
          // Best effort: preserve the host prompt unchanged on failure.
        }
      },

      "tool.execute.after": async (input: {
        tool: string
        sessionID: string
        args: unknown
      }) => {
        try {
          if (input.tool !== MEMORY_TOOL) return
          if (!input.sessionID || !(await isTopLevel(input.sessionID))) return
          if (!input.args || typeof input.args !== "object" || Array.isArray(input.args)) return

          const args = input.args as Record<string, unknown>
          if (typeof args.type !== "string" || !MIRRORED_TYPES.has(args.type)) return
          if (typeof args.title !== "string" || !args.title.trim()) return

          const command = [
            "note",
            "--project",
            project,
            "--source",
            "hook-post",
          ]
          if (typeof args.topic_key === "string" && args.topic_key.trim()) {
            command.push("--scope", args.topic_key.trim())
          }
          if (typeof args.content === "string" && args.content.trim()) {
            command.push("--body", args.content)
          }
          command.push("--", args.title.trim())
          await dependencies.runKb(command, cwd)
        } catch {
          // Mirroring memory into kb is best effort.
        }
      },
    }

    return hooks
  } catch {
    return {}
  }
}) satisfies Plugin

export default Kanban
