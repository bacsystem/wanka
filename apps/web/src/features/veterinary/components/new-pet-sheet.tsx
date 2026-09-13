"use client";

import { ScanLine } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/shared/field";

const sel = (v: string, items: [string, string][]) => (<Select defaultValue={v} items={items.map(([value, label]) => ({ value, label }))}><SelectTrigger className="w-full"><SelectValue /></SelectTrigger><SelectContent>{items.map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent></Select>);

export function NewPetSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [name, setName] = React.useState("");
  const [chip, setChip] = React.useState("");
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader><SheetTitle>Nueva mascota</SheetTitle><SheetDescription>Ficha del paciente vinculada a un propietario existente o nuevo.</SheetDescription></SheetHeader>
        <div className="grid gap-4 px-4 sm:grid-cols-2">
          <Field label="Nombre"><Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Rocky" /></Field>
          <Field label="Especie">{sel("canino", [["canino", "Canino"], ["felino", "Felino"], ["ave", "Ave"], ["roedor", "Roedor"], ["otro", "Otro"]])}</Field>
          <Field label="Raza"><Input placeholder="Golden Retriever" /></Field>
          <Field label="Sexo">{sel("macho", [["macho", "Macho"], ["hembra", "Hembra"]])}</Field>
          <div className="flex items-center justify-between rounded-md border p-3 text-sm"><span>Esterilizado / castrado</span><Switch aria-label="Esterilizado" /></div>
          <Field label="Fecha de nacimiento"><Input type="date" /></Field>
          <Field label="Color / señas"><Input placeholder="Dorado, mancha blanca en pecho" /></Field>
          <Field label="Peso actual (kg)"><Input inputMode="decimal" className="font-mono" placeholder="0.0" /></Field>
          <Field label="Microchip (ISO 11784/11785)" className="sm:col-span-2"><div className="flex gap-2"><Input value={chip} onChange={(e) => setChip(e.target.value)} className="font-mono" placeholder="985112004567890" /><Button type="button" variant="outline" onClick={() => { setChip("985112004599001"); toast.success("Microchip leído desde el lector USB"); }}><ScanLine data-icon="inline-start" /> Leer lector</Button></div></Field>
          <Field label="Propietario" className="sm:col-span-2">{sel("u1", [["u1", "Juan Carlos Pérez Huamán · DNI 45892147"], ["u2", "María Castillo Quispe · DNI 72109843"], ["nuevo", "+ Nuevo propietario (DNI / RUC)"]])}</Field>
          <Field label="Seguro de mascota">{sel("ninguno", [["ninguno", "Sin seguro"], ["rimac", "Rimac Mascotas"], ["pacifico", "Pacífico Pets"]])}</Field>
          <Field label="Foto"><Input type="file" accept="image/*" /></Field>
          <Field label="Alertas médicas" className="sm:col-span-2"><Textarea rows={2} placeholder="Alergias, agresividad, condiciones crónicas…" /></Field>
        </div>
        <SheetFooter><Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button><Button onClick={() => { if (!name.trim()) { toast.error("Ingresa el nombre de la mascota"); return; } toast.success(`${name} registrado · HC-VET-2026-0512`); onOpenChange(false); }}>Guardar mascota</Button></SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
