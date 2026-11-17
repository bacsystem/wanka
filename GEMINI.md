# GEMINI.md

Project context for Gemini CLI and Antigravity. This file is only an import shim: the single source of truth is [`CLAUDE.md`](./CLAUDE.md). Edit `CLAUDE.md`, never this file, and add nothing else here. See "Agent context files" in `CLAUDE.md` and `docs/agent-context.md`.

The first line below is the Gemini CLI import. The second is the Antigravity inline include; Gemini CLI ignores it, and Antigravity treats the first one as a plain path reference. The expanded result must stay at or below 22 000 bytes (`apps/web/src/test/agent-context.test.ts`; Antigravity truncates at 24 000).

@./CLAUDE.md
@[CLAUDE.md](./CLAUDE.md)
