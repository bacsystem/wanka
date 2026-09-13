"use client";

import { Camera, Check, MessageCircle, Scissors } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge, type BadgeTone } from "@/components/shared/status-badge";
import type { GroomingJob } from "../mocks/hospital";

const meta: Record<GroomingJob["status"], { label: string; tone: BadgeTone }> = { en_espera: { label: "En espera", tone: "neutral" }, en_proceso: { label: "En proceso", tone: "warning" }, listo: { label: "Listo para recoger", tone: "success" }, entregado: { label: "Entregado", tone: "info" } };
const next: Record<GroomingJob["status"], GroomingJob["status"]> = { en_espera: "en_proceso", en_proceso: "listo", listo: "entregado", entregado: "entregado" };

export function GroomingScreen({ jobs: initial }: { jobs: GroomingJob[] }) {
  const [jobs, setJobs] = React.useState(initial);
  const groomers = Array.from(new Set(jobs.map((j) => j.groomer)));
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {groomers.map((g) => (
        <SectionCard key={g} title={`Groomer: ${g}`} icon={Scissors} action={<Badge variant="outline">{jobs.filter((j) => j.groomer === g).length} servicios</Badge>} contentClassName="p-0">
          <ul className="divide-y">{jobs.filter((j) => j.groomer === g).map((j) => (
            <li key={j.id} className="flex flex-col gap-2 px-4 py-3 text-sm">
              <div className="flex items-start justify-between gap-2"><div><p className="font-medium"><span className="mr-2 font-mono text-xs">{j.time}</span>{j.pet} <span className="font-normal text-muted-foreground">· {j.breed}</span></p><p className="text-xs text-muted-foreground">{j.owner} · {j.phone} · {j.minutes} min</p></div><StatusBadge tone={meta[j.status].tone} dot label={meta[j.status].label} /></div>
              <div className="flex flex-wrap gap-1">{j.services.map((s) => <Badge key={s} variant="outline">{s}</Badge>)}</div>
              <div className="flex flex-wrap gap-1.5">
                {j.status !== "entregado" ? <Button size="xs" onClick={() => { setJobs((js) => js.map((x) => (x.id === j.id ? { ...x, status: next[x.status] } : x))); toast.success(`${j.pet}: ${meta[next[j.status]].label}`); }}><Check data-icon="inline-start" /> {j.status === "en_espera" ? "Iniciar" : j.status === "en_proceso" ? "Marcar listo" : "Entregar"}</Button> : null}
                {j.status === "listo" ? <Button size="xs" variant="outline" onClick={() => toast.success(`Aviso con foto enviado a ${j.owner}`)}><MessageCircle data-icon="inline-start" /> Avisar al dueño</Button> : null}
                <Button size="xs" variant="ghost" onClick={() => toast.info("Cámara no disponible en la demo")}><Camera data-icon="inline-start" /> Foto</Button>
              </div>
            </li>
          ))}</ul>
        </SectionCard>
      ))}
    </div>
  );
}
