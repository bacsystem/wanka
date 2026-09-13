import type { Metadata } from "next";
import { cookies } from "next/headers";
import { RestaurantDashboard, VeterinaryDashboard } from "@/features/dashboard/components/industry-dashboards";
import { TENANT_COOKIE } from "@/features/auth/lib/tenant-cookie";
import { tenantOptions } from "@/features/auth/mocks/tenants";
import { AlertTriangle, Calendar, Receipt, TrendingUp, Wallet } from "lucide-react";
import { KpiCard } from "@/features/dashboard/components/kpi-card";
import { RecentDocumentsTable } from "@/features/dashboard/components/recent-documents-table";
import { SalesChartCard } from "@/features/dashboard/components/sales-chart";
import { Eye } from "lucide-react";
import Link from "next/link";
import { StatChip } from "@/components/shared/stat-card";
import { StockAlerts } from "@/features/dashboard/components/stock-alerts";
import { SunatStatusCard } from "@/features/dashboard/components/sunat-status-card";
import { formatDate } from "@/lib/format";
import { getDashboardData } from "@/features/dashboard/mocks/dashboard";

export const metadata: Metadata = { title: "Inicio" };

const kpiIcons = {
  sales_today: TrendingUp,
  docs_today: Receipt,
  receivable: Wallet,
  low_stock: AlertTriangle,
} as const;

/** Per-KPI presentation (Stitch: colored icon boxes, chips, sparkline, links). */
const kpiExtras: Record<string, Partial<React.ComponentProps<typeof KpiCard>>> = {
  sales_today: { tone: "default", deltaLabel: "vs. ayer", footer: <Sparkline /> },
  docs_today: { tone: "info", suffix: "documentos", deltaLabel: "vs. ayer", footer: <><StatChip>24 Boletas</StatChip><StatChip>14 Facturas</StatChip></> },
  receivable: { tone: "warning", deltaLabel: "saldo 7 días", footer: <><span className="size-1.5 rounded-full bg-amber-500" />3 clientes con crédito activo</> },
  low_stock: { tone: "danger", suffix: "ítems alerta", emphasizeValue: true, footer: <span className="flex w-full items-center justify-between"><span>Almacén Miraflores</span><Link href="/inventario/stock" className="flex items-center gap-1 font-semibold text-primary hover:underline">Ver alertas <Eye className="size-3" /></Link></span> },
};

function Sparkline() {
  const pts = [8, 9, 7, 10, 9, 12, 11, 13, 12, 15, 14, 16];
  const max = Math.max(...pts);
  const d = pts.map((p, i) => `${(i / (pts.length - 1)) * 100},${20 - (p / max) * 18}`).join(" ");
  return <svg viewBox="0 0 100 20" preserveAspectRatio="none" className="h-5 w-full text-primary" aria-hidden><polyline points={d} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" /></svg>;
}

export default async function DashboardPage() {
  const data = await getDashboardData();
  const today = data.sales30d.at(-1)?.date ?? "";
  const tenantId = (await cookies()).get(TENANT_COOKIE)?.value;
  const industry = tenantOptions.find((t) => t.id === tenantId)?.industry ?? "odontologia";
  const header = (
    <div className="flex flex-wrap items-end justify-between gap-2">
      <div>
        <p className="text-xs text-muted-foreground">Panel de operaciones</p>
        <h1 className="text-xl font-semibold tracking-tight">Inicio</h1>
      </div>
      <span className="inline-flex items-center gap-1.5 rounded-md border bg-card px-2.5 py-1 text-xs text-muted-foreground">
        <Calendar className="size-3.5" strokeWidth={1.5} aria-hidden />
        Hoy, {formatDate(today)}
      </span>
    </div>
  );
  if (industry === "restaurante") return <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">{header}<RestaurantDashboard data={data} /></div>;
  if (industry === "veterinaria") return <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">{header}<VeterinaryDashboard data={data} /></div>;

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <p className="text-xs text-muted-foreground">Panel de operaciones</p>
          <h1 className="text-xl font-semibold tracking-tight">Inicio</h1>
        </div>
        <span className="inline-flex items-center gap-1.5 rounded-md border bg-card px-2.5 py-1 text-xs text-muted-foreground">
          <Calendar className="size-3.5" strokeWidth={1.5} aria-hidden />
          Hoy, {formatDate(today)}
        </span>
      </div>

      {/* KPIs: horizontal scroll on mobile, grid from sm up */}
      <section aria-label="Indicadores del día" className="-mx-4 overflow-x-auto px-4 md:mx-0 md:px-0">
        <div className="grid min-w-max auto-cols-[minmax(12rem,1fr)] grid-flow-col gap-3 sm:min-w-0 sm:grid-flow-row sm:grid-cols-2 md:gap-4 xl:grid-cols-4">
          {data.kpis.map((kpi) => {
            const extra = kpiExtras[kpi.id] ?? {};
            return <KpiCard key={kpi.id} kpi={kpi} icon={kpiIcons[kpi.id as keyof typeof kpiIcons] ?? TrendingUp} {...extra} />;
          })}
        </div>
      </section>

      <div className="grid items-start gap-4 md:gap-6 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 md:gap-6 xl:col-span-2">
          <SalesChartCard data={data.sales30d} />
          <RecentDocumentsTable documents={data.recentDocuments} />
        </div>
        <div className="flex min-w-0 flex-col gap-4 md:gap-6">
          <StockAlerts alerts={data.stockAlerts} />
          <SunatStatusCard queue={data.sunatQueue} />
        </div>
      </div>
    </div>
  );
}
