# Wanka

SaaS de gestión empresarial multirubro para Perú (restaurantes, veterinarias, odontologías, retail): clientes, catálogo, inventario multialmacén, ventas/POS, facturación electrónica SUNAT y reportes, más una consola de administración de la plataforma.

Monorepo pnpm. Aplicación: `apps/web` (Next.js 16, React 19, TypeScript, Tailwind v4, shadcn/ui sobre Base UI).

```bash
pnpm install
pnpm dev          # http://localhost:3000
pnpm lint && pnpm typecheck && pnpm test
pnpm test:e2e     # Playwright (desktop / tablet / mobile)
```

Guía para agentes y convenciones de UI: [`CLAUDE.md`](./CLAUDE.md). Auditoría de estandarización: [`docs/audit-ui-2026-09-13.md`](./docs/audit-ui-2026-09-13.md).
