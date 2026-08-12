import { describe, expect, test } from "bun:test"
import { readFileSync } from "fs"
import { join } from "path"

describe("Threat matrix: doc-like paths", () => {
  test("Makefile only installs allowlisted adapter files", () => {
    const makefile = readFileSync(join(import.meta.dir, "../../Makefile"), "utf-8")

    const installLines = makefile
      .split("\n")
      .filter((line) => line.includes("install -m 0644") || line.includes("install -m 0755"))

    expect(installLines.length).toBeGreaterThan(0)

    // Exact paths, not a pattern. Adding an adapter file to the installer has to
    // be a deliberate edit here, so the installer can never be widened into a
    // way to place arbitrary files in a user's home directory.
    const allowlist = new Set([
      "integrations/opencode/kanban.ts",
      "integrations/claude/kanban.sh",
      "integrations/claude/stop-hook.sh",
    ])
    for (const line of installLines) {
      const match = line.match(/integrations\/[^/]+\/[\w.-]+/)
      if (!match) {
        throw new Error(`install line matched no integration path: ${line}`)
      }
      if (!allowlist.has(match[0])) {
        throw new Error(`install line targets a path outside the allowlist: ${match[0]}`)
      }
    }
  })

  test("adapter source never infers executable from doc-like filename", () => {
    const source = readFileSync(join(import.meta.dir, "kanban.ts"), "utf-8")
    const dangerous = ["requirements.txt", "CMakeLists.txt", ".md", ".mdx", "README.sh"]
    for (const name of dangerous) {
      expect(source).not.toContain(name)
    }
  })
})
