"use client";

import { RefreshCw } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { Field } from "@/components/shared/field";
import { cn } from "@/lib/utils";

const kinds = [["sede", "Sede con atención al público"], ["almacen", "Almacén"], ["subalmacen", "Subalmacén"], ["cuarentena", "Cuarentena / mermas"]];
const sel = (v: string, items: [string, string][], onChange: (v: string) => void) => (
  <Select value={v} onValueChange={(x) => onChange(String(x))} items={items.map(([value, label]) => ({ value, label }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{items.map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent></Select>
);

export function NewWarehouseSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [kind, setKind] = React.useState("almacen");
  const [name, setName] = React.useState("");
  const [dep, setDep] = React.useState("15"); const [prov, setProv] = React.useState("1501"); const [dist, setDist] = React.useState("150131");
  const [annex, setAnnex] = React.useState("0002");
  const [emit, setEmit] = React.useState(kind === "sede");
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader><SheetTitle>Nuevo almacén / sede</SheetTitle><SheetDescription>Se vincula a un establecimiento anexo declarado en la ficha RUC.</SheetDescription></SheetHeader>
        <div className="grid gap-4 px-4 sm:grid-cols-2">
          <Field label="Tipo" className="sm:col-span-2"><RadioGroup value={kind} onValueChange={(v) => { setKind(String(v)); setEmit(v === "sede"); }} className="grid grid-cols-2 gap-2">{kinds.map(([v, l]) => <label key={v} className={cn("flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm", kind === v && "border-primary bg-accent/40")}><RadioGroupItem value={v} /> {l}</label>)}</RadioGroup></Field>
          <Field label="Nombre" className="sm:col-span-2"><Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Almacén Surquillo logístico" /></Field>
          <Field label="Sede padre">{sel("miraflores", [["miraflores", "Sede Miraflores (principal)"], ["sanisidro", "Sede San Isidro"], ["ninguna", "Ninguna (nueva sede)"]], () => {})}</Field>
          <Field label="Responsable de custodia"><Input placeholder="Q.F. Andrea Benavides" /></Field>
          <Field label="Dirección" className="sm:col-span-2"><Input placeholder="Av. Angamos Este 1550" /></Field>
          <Field label="Departamento">{sel(dep, [["15", "Lima"], ["04", "Arequipa"], ["13", "La Libertad"]], setDep)}</Field>
          <Field label="Provincia">{sel(prov, [["1501", "Lima"], ["1507", "Callao"]], setProv)}</Field>
          <Field label="Distrito">{sel(dist, [["150131", "San Isidro"], ["150122", "Miraflores"], ["150141", "Surquillo"]], setDist)}</Field>
          <Field label="Ubigeo"><Input readOnly value={dist} className="font-mono" /></Field>
          <Field label="Código de establecimiento anexo SUNAT" help="Debe existir en la ficha RUC."><div className="flex gap-2"><Input value={annex} onChange={(e) => setAnnex(e.target.value.replace(/\D/g, "").slice(0, 4))} className="font-mono" /><Button type="button" variant="outline" onClick={() => toast.success(`Ficha RUC sincronizada · anexo ${annex} válido`)}><RefreshCw data-icon="inline-start" /> Sincronizar</Button></div></Field>
          <Field label="Series GRE asignadas"><Input defaultValue="T002" className="font-mono uppercase" /></Field>
          <Field label="Control de acceso">{sel("todos", [["todos", "Todo el personal autorizado de la sede"], ["regente", "Solo regente farmacéutico"], ["logistica", "Solo logística"]], () => {})}</Field>
          <div className="flex items-center justify-between rounded-md border p-3 sm:col-span-2"><div><p className="text-sm font-medium">Permite emisión de comprobantes</p><p className="text-xs text-muted-foreground">Habilita series de boleta/factura en este establecimiento.</p></div><Switch checked={emit} onCheckedChange={setEmit} aria-label="Permite emisión" /></div>
        </div>
        <SheetFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button><Button onClick={() => { if (!name.trim()) { toast.error("Ingresa el nombre"); return; } toast.success(`${name} registrado · anexo ${annex}`); onOpenChange(false); }}>Guardar</Button></SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
