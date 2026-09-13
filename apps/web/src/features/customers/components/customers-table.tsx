"use client";

import { Download, Mail, MessageCircle, MoreHorizontal, Phone, ShieldCheck, ShoppingCart, Stethoscope, Upload, UserPlus, X, Zap } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { PageHeader } from "@/components/shared/page-header";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { SearchInput } from "@/components/shared/toolbar";
import { formatCurrency, formatDate } from "@/lib/format";
import { initials } from "@/lib/tenant";
import type { Customer } from "@/types/domain";
import { FilterBar } from "@/components/shared/filter-bar";

const fiscal = {
  al_dia: { label: "Al día", tone: "success" as const },
  con_deuda: { label: "Con deuda", tone: "warning" as const },
  inactivo: { label: "Inactivo", tone: "neutral" as const },
};
const statusOptions = [{ value: "all", label: "Estado: todos" }, { value: "al_dia", label: "Al día" }, { value: "con_deuda", label: "Con deuda" }, { value: "inactivo", label: "Inactivo" }];

export function CustomersScreen({ customers, kpis }: { customers: Customer[]; kpis: React.ReactNode }) {
  const [tab, setTab] = React.useState("all");
  const [query, setQuery] = React.useState("");
  const [status, setStatus] = React.useState("all");
  const [open, setOpen] = React.useState(false);

  const rows = customers.filter((c) => {
    const s = query.trim().toLowerCase();
    return (tab === "all" || c.documentType === tab) && (status === "all" || c.fiscalStatus === status) &&
      (!s || c.name.toLowerCase().includes(s) || c.documentNumber.includes(s) || c.email?.toLowerCase().includes(s));
  });
  const counts = { all: customers.length, DNI: customers.filter((c) => c.documentType === "DNI").length, RUC: customers.filter((c) => c.documentType === "RUC").length };

  const columns: Column<Customer>[] = [
    {
      key: "name", header: "Cliente / paciente / razón social",
      cell: (c) => (
        <div className="flex items-center gap-2.5">
          <InitialsAvatar initials={initials(c.name)} tone="primary" />
          <TwoLine primary={<Link href={`/clientes/${c.id}`} className="flex items-center gap-1 text-sm font-bold hover:text-primary hover:underline">{c.name}{c.fiscalStatus !== "inactivo" ? <ShieldCheck className="size-3.5 text-primary" aria-label="Validado" /> : null}</Link>} secondary={c.clinicalRecord ? `${c.clinicalRecord}` : c.segment} />
        </div>
      ),
    },
    { key: "doc", header: "Documento identidad", cell: (c) => <span className="flex items-center gap-2"><Badge variant="outline" className="rounded bg-muted font-semibold">{c.documentType}</Badge><span className="font-mono text-sm font-medium tabular-nums">{c.documentNumber}</span></span> },
    { key: "contact", header: "Contacto / canal", cell: (c) => <TwoLine primary={<span className="flex items-center gap-1">{c.phone ?? "—"}{c.phone ? <MessageCircle className="size-3 text-primary" /> : null}</span>} secondary={c.email} /> },
    { key: "segment", header: "Segmentación", cell: (c) => <StatusBadge tone={c.segment?.includes("Convenio") ? "info" : c.segment?.includes("frecuente") ? "success" : "neutral"} label={c.segment ?? "—"} className="rounded-md" /> },
    { key: "last", header: "Última operación", cell: (c) => <span className="font-mono tabular-nums">{c.lastOperation ? formatDate(c.lastOperation) : "—"}</span> },
    { key: "billed", header: "Facturado (S/)", align: "right", cell: (c) => <span className="font-mono font-medium tabular-nums">{formatCurrency(c.billedTotal)}</span> },
    { key: "fiscal", header: "Estado", cell: (c) => <StatusBadge tone={fiscal[c.fiscalStatus].tone} dot label={fiscal[c.fiscalStatus].label} /> },
    {
      key: "actions", header: <span className="sr-only">Acciones</span>, align: "right",
      cell: (c) => (
        <div className="flex justify-end gap-0.5">
          <Button variant="ghost" size="icon-sm" aria-label="WhatsApp" onClick={() => toast.info(`Abriendo WhatsApp de ${c.name}`)}><MessageCircle /></Button>
          <Button variant="ghost" size="icon-sm" aria-label="Nueva venta" render={<Link href="/ventas/nueva" />} nativeButton={false}><ShoppingCart /></Button>
          {c.clinicalRecord ? <Button variant="ghost" size="icon-sm" aria-label="Historia clínica" render={<Link href={`/clientes/${c.id}/historia`} />} nativeButton={false}><Stethoscope /></Button> : null}
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="Más acciones" />}><MoreHorizontal /></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem><Phone /> Llamar</DropdownMenuItem>
              <DropdownMenuItem><Mail /> Enviar estado de cuenta</DropdownMenuItem>
              <DropdownMenuItem>Editar datos</DropdownMenuItem>
              <DropdownMenuItem variant="destructive">Desactivar</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        eyebrow="Gestión comercial · Directorio de clientes y pacientes"
        title="Directorio de clientes y pacientes"
        description="Sincronización en tiempo real de padrón RUC y DNI, historial clínico odontológico y estados de cuenta corriente."
        status={<StatusBadge tone="info" icon={ShieldCheck} label="RENIEC & SUNAT online" />}
        actions={
          <>
            <Button variant="outline"><Download data-icon="inline-start" /> Exportar CSV</Button>
            <Button variant="outline"><Upload data-icon="inline-start" /> Importar</Button>
            <Button onClick={() => setOpen(true)}><UserPlus data-icon="inline-start" /> Nuevo cliente <kbd className="ml-1 rounded bg-primary-foreground/15 px-1 font-mono text-[10px]">F3</kbd></Button>
          </>
        }
      />
      {kpis}

      <FilterBar stack>
        <div className="flex flex-wrap items-center gap-2">
          <SearchInput placeholder="Buscar por nombre, DNI, RUC, teléfono o N° historia clínica…" value={query} onChange={setQuery} className="flex-1 sm:max-w-none" />
          <Tabs value={tab} onValueChange={(v) => setTab(String(v))}>
            <TabsList className="rounded-lg bg-muted p-1">
              <TabsTrigger value="all" className="rounded-md px-3 font-semibold data-active:text-primary">Todos ({counts.all})</TabsTrigger>
              <TabsTrigger value="DNI" className="rounded-md px-3 font-semibold data-active:text-primary">DNI ({counts.DNI})</TabsTrigger>
              <TabsTrigger value="RUC" className="rounded-md px-3 font-semibold data-active:text-primary">RUC ({counts.RUC})</TabsTrigger>
            </TabsList>
          </Tabs>
          <Select value={status} onValueChange={(v) => setStatus(String(v))} items={statusOptions}>
            <SelectTrigger className="w-40 bg-muted/50" aria-label="Estado"><SelectValue /></SelectTrigger>
            <SelectContent>{statusOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
          </Select>
        </div>
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          Filtros activos:
          <span className="flex items-center gap-1 rounded bg-muted px-2 py-0.5 font-medium text-foreground">Odontología <X className="size-3" /></span>
          <span className="flex items-center gap-1 rounded bg-muted px-2 py-0.5 font-medium text-foreground">{status === "all" ? "Activos" : statusOptions.find((o) => o.value === status)?.label} <button type="button" aria-label="Quitar filtro" onClick={() => setStatus("all")}><X className="size-3" /></button></span>
          <button type="button" className="font-medium text-primary hover:underline" onClick={() => { setQuery(""); setStatus("all"); setTab("all"); }}>Limpiar filtros</button>
        </div>
      </FilterBar>

      <SectionCard title={<span className="sr-only">Directorio</span>} className="[&>div:first-child]:hidden" contentClassName="p-0">
        <DataTable
          columns={columns} rows={rows} rowKey={(c) => c.id} minWidth="1000px"
          mobileCard={(c) => (
            <Link href={`/clientes/${c.id}`} className="flex items-center gap-3">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent font-mono text-xs font-semibold text-accent-foreground">{initials(c.name)}</span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{c.name}</p>
                <p className="font-mono text-xs text-muted-foreground tabular-nums">{c.documentType} {c.documentNumber}</p>
                <p className="text-xs text-muted-foreground">{c.segment}</p>
              </div>
              <div className="flex flex-col items-end gap-1">
                <span className="font-mono text-sm font-medium tabular-nums">{formatCurrency(c.billedTotal)}</span>
                <StatusBadge tone={fiscal[c.fiscalStatus].tone} dot label={fiscal[c.fiscalStatus].label} />
              </div>
            </Link>
          )}
        />
        <PaginationBar from={1} to={rows.length} total={1248} label="clientes" />
      </SectionCard>

      <NewCustomerSheet open={open} onOpenChange={setOpen} />
    </>
  );
}

function NewCustomerSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [kind, setKind] = React.useState("DNI");
  const [doc, setDoc] = React.useState("");
  const [name, setName] = React.useState("");
  function lookup() {
    if ((kind === "DNI" && doc.length !== 8) || (kind === "RUC" && doc.length !== 11)) {
      toast.error(kind === "DNI" ? "El DNI debe tener 8 dígitos" : "El RUC debe tener 11 dígitos");
      return;
    }
    setName(kind === "DNI" ? "ROSA ELENA TORRES VILLANUEVA" : "CORPORACIÓN ANDINA DE SALUD S.A.C.");
    toast.success(`${kind === "DNI" ? "RENIEC" : "SUNAT"}: datos encontrados`);
  }
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Registrar nuevo cliente / paciente</SheetTitle>
          <SheetDescription>Consulta automática al padrón RENIEC / SUNAT.</SheetDescription>
        </SheetHeader>
        <div className="grid gap-4 px-4 sm:grid-cols-2">
          <Field label="Tipo de personería" className="sm:col-span-2">
            <RadioGroup value={kind} onValueChange={(v) => setKind(String(v))} className="grid grid-cols-3 gap-2">
              {[["DNI", "DNI (natural)"], ["RUC", "RUC (empresa)"], ["CE", "Carné extranjería"]].map(([v, l]) => (
                <label key={v} className="flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm has-data-[state=checked]:border-primary has-data-checked:border-primary">
                  <RadioGroupItem value={v} /> {l}
                </label>
              ))}
            </RadioGroup>
          </Field>
          <Field label={`Número de ${kind}`} help={kind === "DNI" ? "8 dígitos · RENIEC conectado" : kind === "RUC" ? "11 dígitos · SUNAT conectado" : undefined}>
            <div className="flex gap-2">
              <Input value={doc} onChange={(e) => setDoc(e.target.value.replace(/\D/g, ""))} inputMode="numeric" className="font-mono" placeholder={kind === "DNI" ? "72109843" : "20601234567"} />
              <Button type="button" variant="outline" onClick={lookup}><Zap data-icon="inline-start" /> Consultar</Button>
            </div>
          </Field>
          <Field label={kind === "RUC" ? "Razón social" : "Nombres y apellidos"}><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="Teléfono móvil (WhatsApp)"><Input placeholder="+51 9xx xxx xxx" /></Field>
          <Field label="Correo para comprobantes (CPE)"><Input type="email" placeholder="correo@dominio.pe" /></Field>
          {kind === "RUC" ? (
            <Field label="Dirección fiscal" className="sm:col-span-2"><Input placeholder="Se completa desde SUNAT" /></Field>
          ) : (
            <>
              <Field label="Fecha de nacimiento"><Input type="date" /></Field>
              <Field label="Aseguradora / EPS">
                <Select defaultValue="ninguna" items={[{ value: "ninguna", label: "Ninguna" }, { value: "rimac", label: "Rimac EPS" }, { value: "pacifico", label: "Pacífico EPS" }]}>
                  <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent><SelectItem value="ninguna">Ninguna</SelectItem><SelectItem value="rimac">Rimac EPS</SelectItem><SelectItem value="pacifico">Pacífico EPS</SelectItem></SelectContent>
                </Select>
              </Field>
              <Field label="Datos adicionales (alergias / medicación crónica)" className="sm:col-span-2"><Textarea rows={2} placeholder="Opcional · visible en la historia clínica" /></Field>
            </>
          )}
        </div>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
          <Button onClick={() => { toast.success(`Cliente ${name || "nuevo"} guardado`); onOpenChange(false); }}>Guardar y abrir venta</Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
