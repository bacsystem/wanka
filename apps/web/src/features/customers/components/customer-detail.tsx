"use client";

import { AlertTriangle, ArrowLeft, ArrowRight, Ban, Cake, CalendarPlus, CalendarClock, CheckCircle2, ClipboardList, FileDown, FileText, Info, LayoutGrid, Mail, MapPin, MessageCircle, MoreVertical, Pencil, Phone, Printer, Receipt, Send, ShieldCheck, ShieldPlus, ShoppingCart, Stethoscope, UserRound, Wallet } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { EntityHeader } from "@/components/shared/entity-header";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DataTable, type Column } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge, Tag } from "@/components/shared/status-badge";
import { DocumentTypeBadge } from "@/features/documents/components/document-type-badge";
import { SunatStatusBadge } from "@/features/documents/components/sunat-status-badge";
import { formatCurrency, formatDate } from "@/lib/format";
import { initials } from "@/lib/tenant";
import type { Customer } from "@/types/domain";
import type { CustomerDetail as Detail } from "../mocks/customer-detail";

type Tab = "resumen" | "comprobantes" | "citas" | "cuenta" | "documentos";

export function CustomerDetail({ customer, detail }: { customer: Customer; detail: Detail }) {
  const router = useRouter();
  const [tab, setTab] = React.useState<Tab>("resumen");
  const age = 28;
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <nav className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground" aria-label="Ruta">
        <span className="flex flex-wrap items-center gap-1.5"><Link href="/clientes" className="flex items-center gap-1 hover:text-primary"><ArrowLeft className="size-3.5" /> Clientes</Link><span>/</span><Link href="/clientes" className="hover:text-primary">Directorio</Link><span>/</span><span className="font-semibold text-foreground">{customer.name}</span>{customer.clinicalRecord ? <span className="rounded bg-muted px-1.5 py-0.5 font-mono font-medium text-foreground">{customer.clinicalRecord}</span> : null}</span>
        <span className="hidden items-center gap-1.5 rounded-md bg-muted px-2.5 py-1 font-medium lg:flex"><span className="size-1.5 rounded-full bg-emerald-500" /> RUC: 20608941235 • Sede Miraflores</span>
      </nav>

      <EntityHeader
        avatar={<span className="relative flex size-16 shrink-0 items-center justify-center rounded-xl bg-accent text-2xl font-bold text-primary">{initials(customer.name)}<span className="absolute -right-1 -bottom-1 flex size-5 items-center justify-center rounded-md bg-primary text-primary-foreground"><Stethoscope className="size-3" /></span></span>}
        title={customer.name}
        chips={<><StatusBadge tone="success" icon={ShieldCheck} label={`${customer.documentType} ${customer.documentNumber} (RENIEC OK)`} />{customer.clinicalRecord ? <Tag label={customer.clinicalRecord} className="font-mono" /> : null}</>}
        meta={<>
          <p className="flex flex-wrap gap-x-4 gap-y-1"><span className="flex items-center gap-1"><Cake className="size-3.5" /> {age} años (14/05/1998)</span>{customer.phone ? <span className="flex items-center gap-1"><Phone className="size-3.5" /> {customer.phone}</span> : null}{customer.email ? <span className="flex items-center gap-1"><Mail className="size-3.5" /> {customer.email}</span> : null}</p>
          <div className="flex flex-wrap gap-1.5">{detail.billing.insurer ? <StatusBadge tone="info" icon={ShieldPlus} label={`Convenio ${detail.billing.insurer.replace(" Seguros", "")} (${detail.billing.coverage})`} /> : null}<StatusBadge tone="success" dot label={customer.segment ?? "Paciente"} /><StatusBadge tone="success" dot label="Al día (sin deuda)" /></div>
        </>}
        actions={<>
          <Button render={<Link href={`/ventas/nueva?cliente=${customer.id}`} />} nativeButton={false}><ShoppingCart data-icon="inline-start" /> Nueva venta (POS)</Button>
          <Button variant="secondary" render={<Link href="/agenda" />} nativeButton={false}><CalendarPlus data-icon="inline-start" /> Agendar cita</Button>
          <Button variant="outline" onClick={() => toast.info("Abriendo WhatsApp…")}><MessageCircle data-icon="inline-start" /> WhatsApp</Button>
          <Button variant="outline"><Pencil data-icon="inline-start" /> Editar</Button>
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button variant="outline" size="icon" aria-label="Más acciones" />}><MoreVertical /></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem><FileText /> Estado de cuenta detallado</DropdownMenuItem>
              <DropdownMenuItem><FileDown /> Descargar ficha PDF</DropdownMenuItem>
              <DropdownMenuItem><Printer /> Imprimir odontograma</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive"><Ban /> Desactivar paciente</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </>}
      />

      <StatGrid>
        <StatCard label="Facturado total" value={formatCurrency(detail.kpis.billed)} icon={Wallet} footer={<span className="flex w-full justify-between"><span>{detail.kpis.visits} atenciones en 2026</span><span className="font-semibold text-primary">100% cobrado</span></span>} />
        <StatCard label="Deuda pendiente" value={formatCurrency(detail.kpis.debt)} icon={CheckCircle2} tone="success" emphasizeValue footer={<span className="flex w-full justify-between"><span>Límite de crédito:</span><span className="font-mono text-foreground">{formatCurrency(detail.kpis.creditLimit)}</span></span>} />
        <StatCard label="Total citas" value={String(detail.kpis.appointments)} suffix="citas" icon={CalendarClock} tone="info" hint={<StatusBadge tone="success" label={`Asistencia ${detail.kpis.attendance}%`} className="rounded-md" />} footer={`Última visita: ${formatDate(detail.kpis.lastVisit)}`} />
        <StatCard label="Ticket promedio" value={formatCurrency(detail.kpis.avgTicket)} icon={ClipboardList} tone="info" footer="Por tratamiento clínico" />
      </StatGrid>

      <Tabs value={tab} onValueChange={(v) => (v === "hc" ? router.push(`/clientes/${customer.id}/historia`) : setTab(v as Tab))} className="min-w-0 max-w-full">
        <TabsList variant="card">
          <TabsTrigger value="resumen"><LayoutGrid /> Resumen</TabsTrigger>
          <TabsTrigger value="comprobantes"><Receipt /> Comprobantes <span className="rounded bg-muted px-1.5 font-mono text-[11px]">{detail.documents.length}</span></TabsTrigger>
          <TabsTrigger value="citas"><CalendarClock /> Citas y turnos <span className="rounded bg-muted px-1.5 font-mono text-[11px]">{detail.kpis.appointments}</span></TabsTrigger>
          <TabsTrigger value="hc" className="h-9 gap-2 rounded-lg px-3 font-semibold"><Stethoscope /> Historia clínica y odontograma</TabsTrigger>
          <TabsTrigger value="cuenta"><Wallet /> Cuenta corriente y pagos</TabsTrigger>
          <TabsTrigger value="documentos"><FileText /> Documentos y consentimientos</TabsTrigger>
        </TabsList>
      </Tabs>

      {tab === "resumen" ? <Summary detail={detail} customer={customer} /> : null}
      {tab === "comprobantes" ? <DocumentsTable docs={detail.documents} /> : null}
      {tab === "citas" ? <AppointmentsTab detail={detail} /> : null}
      {tab === "cuenta" ? <LedgerTab detail={detail} /> : null}
      {tab === "documentos" ? <FilesTab detail={detail} /> : null}
    </div>
  );
}

function DocumentsTable({ docs, compact }: { docs: Detail["documents"]; compact?: boolean }) {
  const columns: Column<Detail["documents"][number]>[] = [
    { key: "type", header: "Tipo / N°", cell: (d) => <div className="flex items-center gap-2"><DocumentTypeBadge type={d.type} /><span className="font-mono text-xs">{d.number}</span>{d.note ? <span className="text-xs text-muted-foreground">{d.note}</span> : null}</div> },
    { key: "date", header: "Emisión", cell: (d) => <span className="font-mono tabular-nums">{formatDate(d.date)}</span> },
    { key: "total", header: "Total", align: "right", cell: (d) => <span className="font-mono font-medium tabular-nums">{formatCurrency(d.total)}</span> },
    { key: "status", header: "Estado SUNAT", cell: (d) => <SunatStatusBadge status={d.status} /> },
    { key: "files", header: "Archivos", align: "right", cell: () => <div className="flex justify-end gap-0.5"><Button variant="ghost" size="icon-sm" aria-label="PDF"><FileDown /></Button><Button variant="ghost" size="icon-sm" aria-label="XML"><FileText /></Button></div> },
  ];
  return (
    <SectionCard title="Comprobantes de pago (CPE)" action={compact ? <Button variant="ghost" size="sm" render={<Link href="/ventas/comprobantes" />} nativeButton={false}>Ver todos <ArrowRight data-icon="inline-end" /></Button> : <StatusBadge tone="success" dot label="SUNAT OSE conectado" />} contentClassName="p-0">
      <DataTable columns={columns} rows={docs} rowKey={(d) => d.number} minWidth="640px" mobileCard={(d) => (<div className="flex items-center justify-between gap-3"><div><p className="font-mono text-xs">{d.number}</p><p className="text-xs text-muted-foreground">{formatDate(d.date)}</p></div><div className="flex flex-col items-end gap-1"><span className="font-mono text-sm font-medium">{formatCurrency(d.total)}</span><SunatStatusBadge status={d.status} /></div></div>)} />
    </SectionCard>
  );
}

function Summary({ detail, customer }: { detail: Detail; customer: Customer }) {
  return (
    <div className="grid gap-4 xl:grid-cols-3">
      <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
        <DocumentsTable docs={detail.documents} compact />
        <SectionCard title="Tratamientos clínicos recientes" action={<Button variant="ghost" size="sm" render={<Link href={`/clientes/${customer.id}/historia`} />} nativeButton={false}>Ver historial completo <ArrowRight data-icon="inline-end" /></Button>} contentClassName="p-0">
          <ul className="divide-y">
            {detail.treatments.map((t) => (
              <li key={t.name} className="flex items-start gap-3 px-4 py-3 text-sm"><span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-accent text-accent-foreground"><Stethoscope className="size-4" strokeWidth={1.5} /></span><div className="min-w-0 flex-1"><p className="font-medium">{t.name}</p><p className="text-xs text-muted-foreground">Tratante: {t.professional} · {formatDate(t.date)}</p></div><div className="flex flex-col items-end gap-1"><StatusBadge tone="success" label={t.status} /><span className="text-xs text-muted-foreground">{t.payment}</span></div></li>
            ))}
          </ul>
        </SectionCard>
      </div>
      <div className="flex min-w-0 flex-col gap-4">
        {detail.nextAppointment ? (
          <SectionCard title={<span className="text-xs font-bold tracking-wide text-primary uppercase">Próxima cita programada</span>} icon={CalendarClock} className="border-primary/20 bg-accent/40 dark:bg-accent/10" action={<Tag tone="primary" solid label="Confirmada" />} contentClassName="p-4 text-sm">
            <div className="flex flex-col gap-2 rounded-lg bg-card p-4 shadow-xs">
              <p className="text-lg font-bold capitalize">{new Date(detail.nextAppointment.date + "T00:00:00").toLocaleDateString("es-PE", { weekday: "long", day: "2-digit", month: "2-digit", year: "numeric" })} <span className="ml-1 text-primary">{detail.nextAppointment.time} AM</span></p>
              <p className="flex items-center gap-2 text-xs text-muted-foreground"><MapPin className="size-3.5" /> {detail.nextAppointment.place}</p><p className="flex items-center gap-2 text-xs text-muted-foreground"><UserRound className="size-3.5" /> {detail.nextAppointment.professional}</p><p className="flex items-center gap-2 text-xs text-muted-foreground"><ClipboardList className="size-3.5" /> {detail.nextAppointment.reason}</p>
              <div className="mt-2 flex flex-wrap gap-2 [&>*]:min-w-0 [&>*]:flex-1"><Button variant="secondary" onClick={() => toast.info("Reprogramar cita")}>Reprogramar</Button><Button onClick={() => toast.success("Confirmación enviada por WhatsApp")}><Send data-icon="inline-start" /> Confirmar</Button></div>
            </div>
          </SectionCard>
        ) : null}
        <SectionCard title="Alertas clínicas y anamnesis" icon={AlertTriangle} iconTone="danger" contentClassName="flex flex-col gap-2 p-4 text-sm">
          {detail.alerts.map((a) => (
            <div key={a.title} className={a.level === "danger" ? "rounded-md border border-destructive/40 bg-destructive/5 p-3" : "rounded-md border bg-muted/40 p-3"}><p className="flex items-center gap-1.5 font-medium">{a.level === "danger" ? <AlertTriangle className="size-4 text-destructive" /> : <Info className="size-4 text-primary" />}{a.title}</p><p className="mt-1 text-xs text-muted-foreground">{a.text}</p></div>
          ))}
          <p className="text-xs text-muted-foreground">Grupo sanguíneo: <span className="font-medium text-foreground">O positivo (O+)</span></p>
        </SectionCard>
        <SectionCard title="Datos de facturación SUNAT" contentClassName="p-4 text-sm">
          <dl className="grid gap-2 text-xs">
            {[["Tipo de documento", detail.billing.docType], ["Tipo de cliente", detail.billing.kind], ["Dirección fiscal", detail.billing.address], ["Correo CPE", detail.billing.email], ["Aseguradora / EPS", detail.billing.insurer ? `${detail.billing.insurer} · póliza ${detail.billing.policy} · cobertura ${detail.billing.coverage}` : "—"]].map(([k, v]) => (<div key={k}><dt className="text-muted-foreground">{k}</dt><dd className="font-medium">{v}</dd></div>))}
          </dl>
        </SectionCard>
      </div>
    </div>
  );
}

function AppointmentsTab({ detail }: { detail: Detail }) {
  const rows = [
    ...(detail.nextAppointment ? [{ date: detail.nextAppointment.date, time: detail.nextAppointment.time, reason: detail.nextAppointment.reason, pro: "Dra. Claudia Ramos", status: "Confirmada" }] : []),
    ...detail.treatments.map((t) => ({ date: t.date, time: "10:00", reason: t.name, pro: t.professional, status: "Atendida" })),
  ];
  return (
    <SectionCard title="Citas y turnos" contentClassName="p-0">
      <ul className="divide-y">{rows.map((r, i) => (<li key={i} className="flex items-center gap-3 px-4 py-3 text-sm"><span className="w-28 shrink-0 font-mono text-xs tabular-nums">{formatDate(r.date)} {r.time}</span><div className="min-w-0 flex-1"><p className="truncate font-medium">{r.reason}</p><p className="text-xs text-muted-foreground">{r.pro}</p></div><StatusBadge tone={r.status === "Confirmada" ? "info" : "success"} label={r.status} /></li>))}</ul>
    </SectionCard>
  );
}

function LedgerTab({ detail }: { detail: Detail }) {
  return (
    <SectionCard title="Cuenta corriente y pagos" contentClassName="p-0">
      <DataTable columns={[{ key: "date", header: "Fecha", cell: (l) => <span className="font-mono tabular-nums">{formatDate(l.date)}</span> }, { key: "concept", header: "Concepto", cell: (l) => l.concept }, { key: "debit", header: "Cargo", align: "right", cell: (l) => <span className="font-mono tabular-nums">{l.debit ? formatCurrency(l.debit) : ""}</span> }, { key: "credit", header: "Abono", align: "right", cell: (l) => <span className="font-mono text-success tabular-nums">{l.credit ? formatCurrency(l.credit) : ""}</span> }, { key: "balance", header: "Saldo", align: "right", cell: (l) => <span className="font-mono font-medium tabular-nums">{formatCurrency(l.balance)}</span> }]} rows={detail.ledger} rowKey={(l) => `${l.date}-${l.concept}`} minWidth="560px" />
    </SectionCard>
  );
}

function FilesTab({ detail }: { detail: Detail }) {
  return (
    <SectionCard title="Documentos y consentimientos" action={<Button size="sm" variant="outline">Subir documento</Button>} contentClassName="p-0">
      <ul className="divide-y">{detail.files.map((f) => (<li key={f.name} className="flex items-center gap-3 px-4 py-3 text-sm"><FileText className="size-4 shrink-0 text-muted-foreground" /><div className="min-w-0 flex-1"><p className="truncate font-medium">{f.name}</p><p className="text-xs text-muted-foreground">{f.kind} · {formatDate(f.date)} · {f.size}</p></div><Button variant="ghost" size="icon-sm" aria-label="Descargar"><FileDown /></Button></li>))}</ul>
    </SectionCard>
  );
}
