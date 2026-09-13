"use client";

import { PlusCircle, Syringe } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, TwoLine, type Column } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { SearchInput, Toolbar } from "@/components/shared/toolbar";
import { formatCurrency, formatDate } from "@/lib/format";
import type { Pet } from "../mocks/pets";
import { NewPetSheet } from "./new-pet-sheet";

export function PetsList({ pets }: { pets: Pet[] }) {
  const [q, setQ] = React.useState("");
  const [newOpen, setNewOpen] = React.useState(false);
  const rows = pets.filter((p) => !q || p.name.toLowerCase().includes(q.toLowerCase()) || p.owner.toLowerCase().includes(q.toLowerCase()) || p.chip.includes(q));
  const cols: Column<Pet>[] = [
    { key: "n", header: "Mascota", cell: (p) => <TwoLine primary={<Link href={`/mascotas/${p.id}`} className="font-medium hover:text-primary hover:underline">{p.name}</Link>} secondary={`${p.species} · ${p.breed} · ${p.sex}`} /> },
    { key: "hc", header: "HC / chip", cell: (p) => <TwoLine primary={<span className="font-mono text-xs">{p.hc}</span>} secondary={p.chip} mono /> },
    { key: "o", header: "Propietario", cell: (p) => <TwoLine primary={p.owner} secondary={`${p.ownerDoc} · ${p.ownerPhone}`} /> },
    { key: "w", header: "Peso", align: "right", cell: (p) => <span className="font-mono">{p.weightKg.toFixed(1)} kg</span> },
    { key: "lv", header: "Última visita", cell: (p) => <span className="font-mono">{formatDate(p.lastVisit)}</span> },
    { key: "al", header: "Alertas", cell: (p) => p.alerts.length ? <StatusBadge tone="danger" label={`${p.alerts.length} alerta${p.alerts.length === 1 ? "" : "s"}`} /> : <Badge variant="outline">—</Badge> },
    { key: "d", header: "Deuda", align: "right", cell: (p) => <span className={"font-mono " + (p.debt ? "text-warning" : "text-muted-foreground")}>{formatCurrency(p.debt)}</span> },
  ];
  return (
    <>
      <div className="flex justify-end gap-2"><Button variant="outline" onClick={() => toast.info("Lector de microchip USB: en línea")}><Syringe data-icon="inline-start" /> Leer microchip</Button><Button onClick={() => setNewOpen(true)}><PlusCircle data-icon="inline-start" /> Nueva mascota</Button></div>
      <NewPetSheet open={newOpen} onOpenChange={setNewOpen} />
      <SectionCard title="Pacientes" contentClassName="p-0">
        <Toolbar><SearchInput placeholder="Buscar por nombre, propietario o microchip" value={q} onChange={setQ} /></Toolbar>
        <DataTable columns={cols} rows={rows} rowKey={(p) => p.id} minWidth="900px" mobileCard={(p) => <Link href={`/mascotas/${p.id}`} className="flex items-center justify-between gap-3"><div className="min-w-0"><p className="text-sm font-medium">{p.name} <span className="text-xs font-normal text-muted-foreground">· {p.breed}</span></p><p className="text-xs text-muted-foreground">{p.owner}</p></div><span className="font-mono text-xs">{formatDate(p.lastVisit)}</span></Link>} />
      </SectionCard>
    </>
  );
}
