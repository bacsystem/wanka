---
name: Peruvian B2B Operational Engine
colors:
  surface: '#faf8ff'
  surface-dim: '#d2d9f4'
  surface-bright: '#faf8ff'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#f2f3ff'
  surface-container: '#eaedff'
  surface-container-high: '#e2e7ff'
  surface-container-highest: '#dae2fd'
  on-surface: '#131b2e'
  on-surface-variant: '#3e4947'
  inverse-surface: '#283044'
  inverse-on-surface: '#eef0ff'
  outline: '#6e7977'
  outline-variant: '#bdc9c6'
  surface-tint: '#006a63'
  primary: '#005c55'
  on-primary: '#ffffff'
  primary-container: '#0f766e'
  on-primary-container: '#a3faef'
  inverse-primary: '#80d5cb'
  secondary: '#006a61'
  on-secondary: '#ffffff'
  secondary-container: '#86f2e4'
  on-secondary-container: '#006f66'
  tertiary: '#485452'
  on-tertiary: '#ffffff'
  tertiary-container: '#606c6a'
  on-tertiary-container: '#e0edea'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#9cf2e8'
  primary-fixed-dim: '#80d5cb'
  on-primary-fixed: '#00201d'
  on-primary-fixed-variant: '#00504a'
  secondary-fixed: '#89f5e7'
  secondary-fixed-dim: '#6bd8cb'
  on-secondary-fixed: '#00201d'
  on-secondary-fixed-variant: '#005049'
  tertiary-fixed: '#d8e5e2'
  tertiary-fixed-dim: '#bcc9c6'
  on-tertiary-fixed: '#121e1c'
  on-tertiary-fixed-variant: '#3d4947'
  background: '#faf8ff'
  on-background: '#131b2e'
  surface-variant: '#dae2fd'
typography:
  headline-lg:
    fontFamily: Geist
    fontSize: 24px
    fontWeight: '700'
    lineHeight: 32px
    letterSpacing: -0.02em
  headline-md:
    fontFamily: Geist
    fontSize: 20px
    fontWeight: '600'
    lineHeight: 28px
    letterSpacing: -0.015em
  headline-sm:
    fontFamily: Geist
    fontSize: 16px
    fontWeight: '600'
    lineHeight: 24px
    letterSpacing: -0.01em
  body-lg:
    fontFamily: Geist
    fontSize: 15px
    fontWeight: '400'
    lineHeight: 22px
    letterSpacing: 0em
  body-md:
    fontFamily: Geist
    fontSize: 13px
    fontWeight: '400'
    lineHeight: 18px
    letterSpacing: 0em
  body-sm:
    fontFamily: Geist
    fontSize: 12px
    fontWeight: '400'
    lineHeight: 16px
    letterSpacing: 0em
  label-md:
    fontFamily: Geist
    fontSize: 12px
    fontWeight: '500'
    lineHeight: 16px
    letterSpacing: 0.01em
  label-sm:
    fontFamily: Geist
    fontSize: 11px
    fontWeight: '600'
    lineHeight: 14px
    letterSpacing: 0.025em
  data-mono-md:
    fontFamily: Geist Mono
    fontSize: 13px
    fontWeight: '500'
    lineHeight: 18px
    letterSpacing: -0.01em
  data-mono-sm:
    fontFamily: Geist Mono
    fontSize: 11px
    fontWeight: '500'
    lineHeight: 14px
    letterSpacing: 0em
rounded:
  sm: 0.125rem
  DEFAULT: 0.25rem
  md: 0.375rem
  lg: 0.5rem
  xl: 0.75rem
  full: 9999px
spacing:
  gutter: 1rem
  gutter-compact: 0.5rem
  margin: 1.5rem
  margin-mobile: 0.75rem
  space-xs: 0.25rem
  space-sm: 0.5rem
  space-md: 0.75rem
  space-lg: 1rem
  space-xl: 1.5rem
---

## Brand & Style

This design system is engineered for multi-vertical B2B operations in Peru, covering enterprise workflows across restaurants, veterinary centers, dental clinics, and retail establishments. The overarching design direction is strictly operational, high-density, and utilitarian. It rejects decorative marketing patterns, exaggerated whitespace, and unnecessary ornamental visual fluff in favor of high-efficiency data entry, rapid scannability, and relentless administrative clarity.

The visual style merges structured modern corporate architecture with dense data ergonomics. Key operational workflows—such as SUNAT electronic invoicing (boletas, facturas), real-time inventory control, multi-chair or table booking grids, and clinical records—require instant visual comprehension under high-stress business environments. The UI evokes reliability, compliance rigor, structural precision, and frictionless workflow throughput.

## Colors

The system uses an unapologetic enterprise light mode with cool slate undertones to maximize contrast during daylight cashiering and office hours.

- **Primary Teal Base (`#0F766E`)**: Anchors critical user interactions, active navigation states, selected rows, and primary confirmation actions.
- **Teal Focus/Hover (`#0D9488`)**: Dedicated to hover transitions and secondary operational focal points.
- **Teal Subsurface (`#F0FDFA`)**: Applied as the active background for selected table rows, active sidebar modules, and subtle badge containers.
- **Neutrals Foundation**: Pure canvas white (`#FFFFFF`) forms the primary operational background. Secondary panel backing uses slate-50 (`#F8FAFC`), muted borders utilize slate-200 (`#E2E8F0`), secondary labels and de-emphasized metadata run on slate-500 (`#64748B`), and dominant high-contrast text relies on slate-900 (`#0F172A`).
- **SUNAT & Operational Semantics**:
  - Emerald Green (`#10B981` / background `#ECFDF5`): SUNAT "Aceptado", comprobantes emitidos, transacciones pagadas, and in-stock inventory.
  - Amber Gold (`#F59E0B` / background `#FEF3C7`): SUNAT "En Proceso", comprobantes pendientes, cuentas por cobrar, and warning alerts.
  - Crimson Red (`#EF4444` / background `#FEF2F2`): SUNAT "Rechazado", anulaciones, deudas vencidas, and critical stock depletion.
  - Enterprise Blue (`#3B82F6` / background `#EFF6FF`): System notifications, audit trail notes, and contextual guides.

## Typography

The typography uses Inter across display, reading, and label tiers, complemented by JetBrains Mono strictly for numerical accounting, invoice series codes, RUC/DNI documentation, and fiscal amounts.

The type scale deliberately compresses standard web sizes down by 1–2 steps to enable high operational data density:
- Headlines top out at 24px desktop, eliminating sprawling hero-sized titles in favor of compact, functional module headers.
- The workhorse body scale is 13px (`body-md`), ensuring data grids and dense forms display complete information above the fold.
- All monetary outputs in Soles (`S/`) must be rendered with `font-variant-numeric: tabular-nums` or the monospaced token (`data-mono-md` / `data-mono-sm`) to guarantee strict vertical decimal alignment in financial columns.
- Table headers and micro meta-tags use uppercase or tight semibold `label-sm` (11px) with positive letter spacing to maintain legibility when rendered against low-contrast neutral fills.

## Layout & Spacing

The layout adopts a disciplined, responsive operational grid engineered around an 8-point base rhythm, with a 4-point sub-grid for dense element alignments, input internal padding, and table row sizing.

- **Desktop Workspace (1024px+)**: Fixed narrow navigational shell (240px expanded, 64px iconized) coupled with a fluid multi-pane workspace. Inner sections adhere to a 12-column structure with 16px (`1rem`) gutters and 24px (`1.5rem`) outer canvas margins.
- **Data Worksurfaces**: For cashiering points of sale (POS) and scheduling matrixes, gutters tighten to `gutter-compact` (8px / `0.5rem`) to prevent unnecessary dead space between product cards or multi-resource columns.
- **Tablet / Touch Terminal (768px - 1023px)**: Fixed navigation transitions into an off-canvas drawer or top status strip. Layout maintains 16px outer margins, and column layouts collapse to 4 or 6 columns with horizontal scroll preservation for fiscal tables.
- **Mobile Handheld (<768px)**: 12px outer canvas margins (`margin-mobile`). Forms stack into single columns; multi-column balance sheets and SUNAT logs transform into dense collapsible summary cards with direct status pills.

## Elevation & Depth

This system intentionally departs from heavy drop shadows, relying instead on structural 1px low-contrast borders (`#E2E8F0`) and subtle tonal stepping to establish visual order.

- **Level 0 (Flat Canvas)**: Background workspace color `#F8FAFC`.
- **Level 1 (Surface Containers & Cards)**: Pure white `#FFFFFF` bounded by a crisp 1px solid border (`#E2E8F0`). Flat appearance without drop shadows; depth is derived from surface contrast against Level 0.
- **Level 2 (Dropdowns, Popovers, & Context Menus)**: Pure white `#FFFFFF` surface with a 1px solid border (`#CBD5E1`) and a minimal, tight utilitarian shadow: `0 4px 6px -1px rgba(15, 23, 42, 0.08), 0 2px 4px -2px rgba(15, 23, 42, 0.04)`.
- **Level 3 (Modals & Confirmation Sheets)**: Positioned above an overlay tint (`rgba(15, 23, 42, 0.45)`). Bordered with `#E2E8F0` and elevated with `0 10px 15px -3px rgba(15, 23, 42, 0.1), 0 4px 6px -4px rgba(15, 23, 42, 0.05)`.
- **Interactive Focus**: Elevation does not change on element hover. States are communicated through precise border color shifts to `#0F766E` and subtle background tinting.

## Shapes

The design system standardizes on a measured, clean corner radius to balance modern visual polish with high-density data framing:

- **Base Radius (4px / 0.25rem)**: Applied to micro elements, table filter badges, contextual buttons, input fields, and inline dropdown triggers.
- **Container Radius (8px / 0.5rem - `rounded-lg`)**: Applied to standard content cards, data tables, modal shells, and flyout sidebars.
- **Pill Radius (Full rounded / 9999px)**: Strictly reserved for status badges (SUNAT status, stock flags, payment state indicators) and avatar initials.

## Components

All components adhere to dense horizontal and vertical specifications, utilizing Lucide Icons (stroke width: 1.75px) sized strictly at 14px or 16px.

### Buttons
- **Primary**: Solid background `#0F766E`, text `#FFFFFF`, hover `#0D9488`, active `#115E59`. Height: 32px (compact) or 36px (standard). Padding: 0 12px. Font: 13px weight 500.
- **Secondary / Outline**: Background `#FFFFFF`, border 1px solid `#E2E8F0`, text `#0F172A`, hover background `#F8FAFC`, hover border `#CBD5E1`.
- **Ghost / Table Action**: Zero border, text `#64748B`, hover text `#0F172A`, hover background `#F1F5F9`. Compact padding: 4px 8px.

### Input Fields & Selects
- Form inputs feature a 32px height (operational density standard), 8px horizontal padding, 1px solid `#E2E8F0` border, and `#0F172A` text.
- Focus state: 1px border `#0F766E` with a 1px ring offset (`box-shadow: 0 0 0 1px #0F766E`).
- Embedded prefix/suffix elements (e.g., `S/`, RUC search triggers, calendar icons) are anchored in muted slate (`#64748B`) on background `#F8FAFC`.

### Data Tables (Core Operational Component)
- **Header**: Background `#F8FAFC`, border-bottom 1px solid `#E2E8F0`, height 32px. Text: `label-sm` (11px, weight 600, uppercase, color `#64748B`).
- **Rows**: Pure white background, hover background `#F8FAFC`, selected row background `#F0FDFA`. Height: 36px default, 28px in ultra-dense mode.
- **Data Alignment**: Text aligns left; dates and status indicators align center; fiscal totals, quantities, and prices align strictly right using tabular numerical tokens.

### Status Pills
- High-visibility, compact indicators composed of an outer rounded-full shell, a 6px diameter colored solid dot, and 11px medium text.
- **SUNAT Aceptado / Pagado**: Background `#ECFDF5`, text `#065F46`, dot `#10B981`.
- **Pendiente / Por Cobrar**: Background `#FEF3C7`, text `#92400E`, dot `#F59E0B`.
- **Rechazado / Stock Crítico**: Background `#FEF2F2`, text `#991B1B`, dot `#EF4444`.
- **Informativo / En Proceso**: Background `#EFF6FF`, text `#1E40AF`, dot `#3B82F6`.

### Cards & Metrics
- Plain `#FFFFFF` background, 1px solid `#E2E8F0` border, 12px internal padding. KPI headers display uppercase 11px slate-500 labels; KPI numerical values render at 20px semibold tabular figures with right-side percentage variance badges.
