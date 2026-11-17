# Agent context files

`CLAUDE.md` at the repo root is the single source of truth. Every other agent context file only imports it: edit `CLAUDE.md`, never copy its content.

| Tool | What it loads | How `CLAUDE.md` reaches it |
|---|---|---|
| Claude Code | root `CLAUDE.md`; `apps/web/CLAUDE.md` when working under `apps/web` (contains only `@AGENTS.md`) | directly |
| Gemini CLI ([docs](https://geminicli.com/docs/cli/gemini-md/)) | `GEMINI.md` in the workspace and its parents; just-in-time `GEMINI.md` of folders a tool touches (`apps/web/GEMINI.md` → `@./AGENTS.md`) | root `GEMINI.md` line `@./CLAUDE.md` (Gemini CLI import) |
| Antigravity ([docs](https://antigravity.google/docs/rules)) | `AGENTS.md` / `GEMINI.md` of the workspace folder and its parents, never subfolders | root `GEMINI.md` second line: an at-sign followed by a Markdown link to `./CLAUDE.md`, which Antigravity inlines; its plain at-sign path only inserts a reference, and Gemini CLI ignores the Markdown-link form |

- `AGENTS.md` is deliberately **not** in Gemini CLI's `context.fileName`: it would also load the `AGENTS.md` files that packages ship inside `node_modules` (`next` does), which is a prompt-injection surface. There is no `.gemini/settings.json`.
- `apps/web/AGENTS.md` is generated and re-added by `next dev`; do not edit it by hand. Antigravity only applies it when the workspace is opened at `apps/web`, which is why `apps/web/GEMINI.md` has no Markdown-link include. There is no root `AGENTS.md`, to avoid confusing it with that file.
- **Size budget**: Antigravity truncates a rule file above 24 000 bytes after includes. `apps/web/src/test/agent-context.test.ts` (part of `pnpm test`) expands the root `GEMINI.md` and fails above 22 000 bytes. It also fails if `CLAUDE.md` contains the Markdown-link include syntax (Antigravity would expand it), or if a Gemini import points to a missing file. If it fails, move detail from `CLAUDE.md` into a linked doc; don't raise the limit.
