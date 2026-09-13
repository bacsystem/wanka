"use client";

import { Bike, ChefHat, Clock, HeartPulse, PawPrint, Receipt, Stethoscope, Syringe, TrendingUp, UtensilsCrossed } from "lucide-react";
import Link from "next/link";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { RecentDocumentsTable } from "@/features/dashboard/components/recent-documents-table";
import { SunatStatusCard } from "@/features/dashboard/components/sunat-status-card";
import { formatCurrency } from "@/lib/format";
import type { DashboardData } from "@/types/domain";

const cfg = { total: { label: "Ventas", color: "var(--chart-1)" } } satisfies ChartConfig;
const hourly = [8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22].map((h) => ({ h: `${h}:00`, total: [80, 120, 160, 240, 620, 980, 760, 320, 180, 160, 240, 560, 840, 520, 210][h - 8] }));

export function RestaurantDashboard({ data }: { data: DashboardData }) {
  return (
    <>
      <StatGrid>
        <StatCard label="Ventas del turno" value={formatCurrency(4120)} icon={TrendingUp} deltaPercent={9.2} hint="Turno almuerzo · 12:00–17:00" />
        <StatCard label="Mesas ocupadas" value="9 / 22" icon={UtensilsCrossed} hint="41% del salón · 2 por cobrar" />
        <StatCard label="Ticket promedio" value={formatCurrency(68)} icon={Receipt} tone="success" hint="+S/ 4 vs. ayer" />
        <StatCard label="Tiempo medio de atención" value="38 min" icon={Clock} tone="warning" hint="1 comanda > 15 min en cocina" />
      </StatGrid>
      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <SectionCard title="Ventas por hora del día" contentClassName="p-4">
            <ChartContainer config={cfg} className="aspect-auto h-56 w-full"><BarChart data={hourly} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}><CartesianGrid vertical={false} strokeDasharray="3 3" /><XAxis dataKey="h" tickLine={false} axisLine={false} tickMargin={8} minTickGap={16} /><YAxis tickLine={false} axisLine={false} width={40} /><ChartTooltip content={<ChartTooltipContent formatter={(v) => <span className="font-mono">{formatCurrency(Number(v))}</span>} />} /><Bar dataKey="total" fill="var(--color-total)" radius={[3, 3, 0, 0]} isAnimationActive={false} /></BarChart></ChartContainer>
          </SectionCard>
          <RecentDocumentsTable documents={data.recentDocuments} />
        </div>
        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Top platos del día" icon={ChefHat} contentClassName="p-0"><ul className="divide-y text-sm">{[["Ceviche clásico", 42, 1596], ["Arroz con mariscos", 21, 966], ["Chilcano clásico", 38, 836], ["Parihuela especial", 9, 522], ["Causa de cangrejo", 14, 448]].map(([n, q, a]) => <li key={String(n)} className="flex items-center gap-3 px-4 py-2"><span className="flex-1">{n}</span><span className="font-mono text-xs text-muted-foreground">{q} und</span><span className="font-mono text-xs font-medium">{formatCurrency(Number(a))}</span></li>)}</ul></SectionCard>
          <SectionCard title="Insumos críticos" contentClassName="p-0"><ul className="divide-y text-sm">{[["Pescado del día (corvina)", "8 kg", "mín. 15 kg"], ["Limón", "3 kg", "mín. 10 kg"], ["Camote", "6 kg", "mín. 8 kg"]].map(([n, s, m]) => <li key={n} className="flex items-center gap-3 px-4 py-2"><span className="flex-1">{n}</span><span className="font-mono text-xs font-semibold text-destructive">{s}</span><span className="text-xs text-muted-foreground">{m}</span></li>)}</ul><div className="border-t p-2"><Button size="sm" variant="ghost" render={<Link href="/compras/ordenes/nueva" />} nativeButton={false}>Generar orden de compra →</Button></div></SectionCard>
          <SectionCard title="Operación en curso" contentClassName="grid grid-cols-2 gap-2 p-3 text-sm"><Link href="/salon/cocina" className="rounded-md border p-3 hover:bg-muted/40"><p className="text-xs text-muted-foreground">Comandas retrasadas</p><p className="font-mono text-xl font-semibold text-destructive">1</p></Link><Link href="/salon/delivery" className="rounded-md border p-3 hover:bg-muted/40"><p className="text-xs text-muted-foreground">Delivery activos</p><p className="font-mono text-xl font-semibold">4</p></Link></SectionCard>
          <SunatStatusCard queue={data.sunatQueue} />
        </div>
      </div>
    </>
  );
}

export function VeterinaryDashboard({ data }: { data: DashboardData }) {
  const agenda = [["09:00", "Rocky · control dermatológico", "Consultorio 1", "atendido"], ["10:30", "Luna · retiro de sonda", "UCI", "en_curso"], ["11:00", "Max · vacuna séxtuple", "Consultorio 2", "confirmado"], ["12:00", "Kira · control post-quirúrgico", "Consultorio 1", "confirmado"], ["15:30", "Toby · ecografía abdominal", "Imágenes", "agendado"]];
  return (
    <>
      <StatGrid>
        <StatCard label="Consultas hoy" value="18" icon={Stethoscope} hint="4 en espera · 2 consultorios activos" />
        <StatCard label="Ventas del día" value={formatCurrency(2860)} icon={TrendingUp} deltaPercent={6.4} hint="Consultas, petshop y grooming" />
        <StatCard label="Hospitalizados" value="8" icon={HeartPulse} tone="danger" hint="2 críticos · 1 alta prevista" />
        <StatCard label="Vacunas por vencer (30 d)" value="37" icon={Syringe} tone="warning" hint="Recordatorios WhatsApp programados" />
      </StatGrid>
      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <SectionCard title="Agenda del día por consultorio" icon={PawPrint} action={<Button size="sm" variant="ghost" render={<Link href="/agenda" />} nativeButton={false}>Ver agenda →</Button>} contentClassName="p-0"><ul className="divide-y text-sm">{agenda.map(([t, w, r, s]) => <li key={t} className="flex items-center gap-3 px-4 py-2.5"><span className="w-12 font-mono text-xs">{t}</span><span className="flex-1">{w}</span><Badge variant="outline">{r}</Badge><StatusBadge tone={s === "atendido" ? "success" : s === "en_curso" ? "info" : s === "confirmado" ? "neutral" : "warning"} dot label={s === "atendido" ? "Atendido" : s === "en_curso" ? "En curso" : s === "confirmado" ? "Confirmado" : "Agendado"} /></li>)}</ul></SectionCard>
          <RecentDocumentsTable documents={data.recentDocuments} />
        </div>
        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Recordatorios de vacunas" icon={Syringe} contentClassName="grid grid-cols-3 gap-2 p-3 text-center text-sm"><div className="rounded-md border p-2"><p className="text-xs text-muted-foreground">Enviados</p><p className="font-mono text-xl font-semibold">124</p></div><div className="rounded-md border p-2"><p className="text-xs text-muted-foreground">Confirmados</p><p className="font-mono text-xl font-semibold text-success">84</p></div><div className="rounded-md border p-2"><p className="text-xs text-muted-foreground">Sin respuesta</p><p className="font-mono text-xl font-semibold text-warning">40</p></div></SectionCard>
          <SectionCard title="Stock crítico (fármacos y alimentos)" contentClassName="p-0"><ul className="divide-y text-sm">{[["Bordetella KC", "3 dosis", "mín. 10"], ["Royal Canin Gastro 2 kg", "2 und", "mín. 6"], ["Bravecto 20–40 kg", "1 und", "mín. 5"]].map(([n, s, m]) => <li key={n} className="flex items-center gap-3 px-4 py-2"><span className="flex-1">{n}</span><span className="font-mono text-xs font-semibold text-destructive">{s}</span><span className="text-xs text-muted-foreground">{m}</span></li>)}</ul></SectionCard>
          <SectionCard title="Próximos grooming" icon={Bike} contentClassName="p-0"><ul className="divide-y text-sm">{[["12:30", "Coco · baño, corte, uñas", "Karla"], ["14:00", "Lola · baño seco", "Pedro"]].map(([t, w, g]) => <li key={t} className="flex items-center gap-3 px-4 py-2"><span className="font-mono text-xs">{t}</span><span className="flex-1">{w}</span><span className="text-xs text-muted-foreground">{g}</span></li>)}</ul><div className="border-t p-2"><Button size="sm" variant="ghost" render={<Link href="/grooming" />} nativeButton={false}>Agenda de grooming →</Button></div></SectionCard>
          <SunatStatusCard queue={data.sunatQueue} />
        </div>
      </div>
    </>
  );
}
