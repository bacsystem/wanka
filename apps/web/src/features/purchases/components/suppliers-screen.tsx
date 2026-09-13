"use client";

import { History, Mail, MoreHorizontal, Phone, UserPlus, Zap } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InitialsAvatar } from "@/components/shared/initials-avatar";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { Field } from "@/components/shared/field";
import { PaginationBar } from "@/components/shared/pagination";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import { SearchInput, Toolbar } from "@/components/shared/toolbar";
import { formatCurrency } from "@/lib/format";
import { initials } from "@/lib/tenant";
import type { Supplier } from "../mocks/suppliers";

const statusMeta: Record<Supplier["status"], { label: string; tone: BadgeTone }> = { activo: { label: "Activo", tone: "success" }, observado: { label: "Observado", tone: "warning" }, inactivo: { label: "Inactivo", tone: "neutral" } };

export function SuppliersScreen({ suppliers, openNew, onOpenNewChange }: { suppliers: Supplier[]; openNew: boolean; onOpenNewChange: (o: boolean) => void }) {
  const [query, setQuery] = React.useState("");
  const rows = suppliers.filter((s) => { const q = query.trim().toLowerCase(); return !q || s.name.toLowerCase().includes(q) || s.ruc.includes(q) || s.category.toLowerCase().includes(q); });

  const columns: Column<Supplier>[] = [
    { key: "name", header: "Proveedor / RUC", cell: (s) => (
      <div className="flex items-center gap-2.5">
        <InitialsAvatar initials={initials(s.name)} />
        <TwoLine primary={<Link href={`/compras/proveedores/${s.id}`} className="font-medium hover:text-primary hover:underline">{s.name}</Link>} secondary={`RUC ${s.ruc}`} mono />
      </div>
    ), className: "max-w-72" },
    { key: "cat", header: "Rubro", cell: (s) => <Badge variant="outline">{s.category}</Badge> },
    { key: "contact", header: "Contacto", cell: (s) => <TwoLine primary={s.contact} secondary={`${s.phone} · ${s.email}`} /> },
    { key: "terms", header: "Condición de pago", cell: (s) => <span className="text-xs">{s.terms}</span> },
    { key: "year", header: "Compras 2026", align: "right", cell: (s) => <span className="font-mono tabular-nums">{formatCurrency(s.yearTotal)}</span> },
    { key: "pending", header: "Por pagar", align: "right", cell: (s) => <span className={"font-mono font-medium tabular-nums " + (s.pending ? "text-warning" : "text-muted-foreground")}>{formatCurrency(s.pending)}</span> },
    { key: "sunat", header: "SUNAT", cell: (s) => <StatusBadge tone={s.sunat === "habido" ? "success" : "danger"} dot label={s.sunat === "habido" ? "Habido" : "No habido"} /> },
    { key: "status", header: "Estado", cell: (s) => <StatusBadge tone={statusMeta[s.status].tone} label={statusMeta[s.status].label} /> },
    { key: "actions", header: <span className="sr-only">Acciones</span>, align: "right", cell: (s) => (
      <div className="flex justify-end gap-0.5">
        <Button variant="ghost" size="icon-sm" aria-label="Historial" render={<Link href={`/compras/proveedores/${s.id}`} />} nativeButton={false}><History /></Button>
        <Button variant="ghost" size="icon-sm" aria-label="Llamar"><Phone /></Button>
        <Button variant="ghost" size="icon-sm" aria-label="Correo"><Mail /></Button>
        <Button variant="ghost" size="icon-sm" aria-label="Más"><MoreHorizontal /></Button>
      </div>
    ) },
  ];

  return (
    <>
      <SectionCard title="Directorio de proveedores" contentClassName="p-0">
        <Toolbar><SearchInput placeholder="Buscar proveedor, RUC o rubro" value={query} onChange={setQuery} /></Toolbar>
        <DataTable columns={columns} rows={rows} rowKey={(s) => s.id} minWidth="1180px" mobileCard={(s) => (
          <div className="flex items-start gap-3">
            <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{s.name}</p><p className="font-mono text-xs text-muted-foreground">RUC {s.ruc} · {s.category}</p><p className="text-xs text-muted-foreground">{s.terms}</p></div>
            <div className="flex flex-col items-end gap-1"><span className="font-mono text-sm font-medium tabular-nums">{formatCurrency(s.pending)}</span><StatusBadge tone={statusMeta[s.status].tone} label={statusMeta[s.status].label} /></div>
          </div>
        )} />
        <PaginationBar from={1} to={rows.length} total={40} label="proveedores" />
      </SectionCard>
      <Sheet open={openNew} onOpenChange={onOpenNewChange}>
        <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
          <SheetHeader><SheetTitle>Nuevo proveedor</SheetTitle><SheetDescription>Validación automática del RUC contra el padrón SUNAT.</SheetDescription></SheetHeader>
          <div className="grid gap-4 px-4 sm:grid-cols-2">
            <Field label="RUC" help="11 dígitos · SUNAT conectado"><div className="flex gap-2"><Input placeholder="20601234567" className="font-mono" inputMode="numeric" /><Button variant="outline" type="button" onClick={() => toast.success("SUNAT: contribuyente habido")}><Zap data-icon="inline-start" /> Consultar</Button></div></Field>
            <Field label="Razón social"><Input placeholder="Se completa desde SUNAT" /></Field>
            <Field label="Rubro"><Select defaultValue="insumos" items={[{ value: "insumos", label: "Insumos clínicos" }, { value: "medicamentos", label: "Medicamentos" }, { value: "lab", label: "Laboratorio dental" }, { value: "servicios", label: "Servicios" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="insumos">Insumos clínicos</SelectItem><SelectItem value="medicamentos">Medicamentos</SelectItem><SelectItem value="lab">Laboratorio dental</SelectItem><SelectItem value="servicios">Servicios</SelectItem></SelectContent></Select></Field>
            <Field label="Condición de pago"><Select defaultValue="30" items={[{ value: "contado", label: "Contado" }, { value: "15", label: "Crédito 15 días" }, { value: "30", label: "Crédito 30 días" }]}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="contado">Contado</SelectItem><SelectItem value="15">Crédito 15 días</SelectItem><SelectItem value="30">Crédito 30 días</SelectItem></SelectContent></Select></Field>
            <Field label="Contacto"><Input /></Field>
            <Field label="Teléfono"><Input placeholder="+51 9xx xxx xxx" /></Field>
            <Field label="Correo para órdenes" className="sm:col-span-2"><Input type="email" /></Field>
            <Field label="Cuenta de detracciones (Banco de la Nación)" className="sm:col-span-2"><Input placeholder="00-000-000000" className="font-mono" /></Field>
          </div>
          <SheetFooter><Button variant="outline" onClick={() => onOpenNewChange(false)}>Cancelar</Button><Button onClick={() => { toast.success("Proveedor registrado"); onOpenNewChange(false); }}><UserPlus data-icon="inline-start" /> Guardar proveedor</Button></SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  );
}

export function SuppliersPageBody({ suppliers }: { suppliers: Supplier[] }) {
  const [open, setOpen] = React.useState(false);
  return (
    <>
      <div className="flex justify-end"><Button onClick={() => setOpen(true)}><UserPlus data-icon="inline-start" /> Nuevo proveedor</Button></div>
      <SuppliersScreen suppliers={suppliers} openNew={open} onOpenNewChange={setOpen} />
    </>
  );
}
