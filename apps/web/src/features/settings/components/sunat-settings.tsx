"use client";

import { Building2, CheckCircle2, Copy, Eye, FileBadge2, ListOrdered, Pencil, Plug, PlusCircle, Printer, RefreshCw, Save, Upload, Users } from "lucide-react";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/shared/field";
import { DataTable } from "@/components/shared/data-table";
import { SectionCard } from "@/components/shared/section-card";
import { DocumentTypeBadge } from "@/features/documents/components/document-type-badge";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { sunatConfig } from "../mocks/settings";

type Config = typeof sunatConfig;

export function SunatSettings({ config }: { config: Config }) {
  const router = useRouter();
  const [env, setEnv] = React.useState(config.connection.environment);
  const [channel, setChannel] = React.useState(config.connection.channel);
  const [format, setFormat] = React.useState(config.print.format);

  return (
    <Tabs defaultValue="certificado" className="min-w-0 max-w-full gap-4">
      <TabsList variant="card">
        <TabsTrigger value="empresa"><Building2 /> Datos de empresa y sedes</TabsTrigger>
        <TabsTrigger value="certificado"><FileBadge2 /> Certificado digital y SOL SUNAT <span className="size-1.5 rounded-full bg-primary" /></TabsTrigger>
        <TabsTrigger value="series"><ListOrdered /> Series y correlativos</TabsTrigger>
        <TabsTrigger value="impresion"><Printer /> Diseño de impresión (A4 / Ticket 80mm)</TabsTrigger>
        <TabsTrigger value="usuarios" className="h-9 gap-2 rounded-lg px-4 font-semibold" onClick={() => router.push("/configuracion/usuarios")}><Users /> Usuarios y permisos</TabsTrigger>
      </TabsList>

      <TabsContent value="empresa">
        <SectionCard title="Datos de la empresa" contentClassName="grid gap-4 p-4 sm:grid-cols-2">
          <Field label="Razón social" className="sm:col-span-2"><Input defaultValue={config.company.name} /></Field>
          <Field label="RUC"><Input defaultValue={config.company.ruc} className="font-mono" readOnly /></Field>
          <Field label="Régimen tributario"><Input defaultValue={config.company.regime} /></Field>
          <Field label="Domicilio fiscal" className="sm:col-span-2"><Input defaultValue={config.company.address} /></Field>
          <Field label="Ubigeo"><Input defaultValue={config.company.ubigeo} className="font-mono" /></Field>
          <Field label="Rubro principal"><Input defaultValue="Odontología" /></Field>
          <div className="flex justify-end sm:col-span-2"><Button onClick={() => toast.success("Datos de empresa guardados")}><Save data-icon="inline-start" /> Guardar cambios</Button></div>
        </SectionCard>
      </TabsContent>

      <TabsContent value="certificado" className="grid gap-4 xl:grid-cols-2">
        <SectionCard title="Certificado digital tributario" icon={FileBadge2} action={<StatusBadge tone="success" icon={CheckCircle2} label={`Vigente hasta ${formatDate(config.certificate.validUntil)}`} />} contentClassName="flex flex-col gap-3 p-4 text-sm">
          <p className="text-xs text-muted-foreground">Firma digital de comprobantes XML bajo el estándar UBL 2.1.</p>
          <dl className="grid gap-3 sm:grid-cols-2">
            <div><dt className="text-xs text-muted-foreground">Entidad certificadora</dt><dd>{config.certificate.issuer}</dd></div>
            <div><dt className="text-xs text-muted-foreground">Serial</dt><dd className="font-mono text-xs">{config.certificate.serial}</dd></div>
            <div><dt className="text-xs text-muted-foreground">RUC titular</dt><dd className="font-mono">{config.certificate.holderRuc}</dd></div>
            <div><dt className="text-xs text-muted-foreground">Almacenamiento</dt><dd>{config.certificate.storage} · cifrado AES-256 en reposo</dd></div>
            <div className="sm:col-span-2">
              <dt className="text-xs text-muted-foreground">Huella SHA-256</dt>
              <dd className="flex items-start gap-1"><code className="font-mono text-[11px] break-all">{config.certificate.sha256}</code><Button variant="ghost" size="icon-xs" aria-label="Copiar huella" onClick={() => toast.success("Huella copiada")}><Copy /></Button></dd>
            </div>
          </dl>
          <div className="flex flex-wrap gap-2 pt-1">
            <Button variant="outline" onClick={() => toast.info("Selecciona el archivo .pfx / .p12")}><Upload data-icon="inline-start" /> Actualizar / renovar (.pfx / .p12)</Button>
            <Button variant="ghost"><Eye data-icon="inline-start" /> Ver cadena de confianza</Button>
          </div>
        </SectionCard>

        <SectionCard title="Conexión SUNAT / proveedor OSE" icon={Plug} action={<StatusBadge tone="success" dot label={`Operativo · ${config.connection.latencyMs} ms`} />} contentClassName="flex flex-col gap-4 p-4 text-sm">
          <Field label="Canal de timbrado y validación">
            <RadioGroup value={channel} onValueChange={(v) => setChannel(String(v) as Config["connection"]["channel"])} className="grid gap-2">
              {[["sunat", "Envío directo a SUNAT", "Servidor SOAP oficial · sin costo · SLA variable"], ["ose", "Operador OSE homologado", `${config.connection.oseProvider} · SLA 99.9% · CDR en segundos`]].map(([v, l, d]) => (
                <label key={v} className={cn("flex cursor-pointer items-start gap-3 rounded-md border p-3", channel === v && "border-primary bg-accent/40")}>
                  <RadioGroupItem value={v} className="mt-0.5" />
                  <span><span className="block font-medium">{l}</span><span className="block text-xs text-muted-foreground">{d}</span></span>
                </label>
              ))}
            </RadioGroup>
          </Field>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Usuario secundario Clave SOL" help="Debe tener el perfil de facturador electrónico asignado."><Input defaultValue={config.connection.solUser} className="font-mono" /></Field>
            <Field label="Contraseña Clave SOL"><Input type="password" defaultValue="••••••••••" /></Field>
          </div>
          <Field label="Entorno activo de emisión">
            <div className="grid grid-cols-2 gap-2">
              {[["beta", "Pruebas / homologación"], ["produccion", "Producción real"]].map(([v, l]) => (
                <button key={v} type="button" onClick={() => setEnv(v as Config["connection"]["environment"])} aria-pressed={env === v} className={cn("rounded-md border px-3 py-2 text-sm font-medium", env === v && (v === "produccion" ? "border-success bg-success/10 text-success" : "border-warning bg-warning/10 text-warning"))}>{l}</button>
              ))}
            </div>
          </Field>
          <div className="flex flex-wrap items-center justify-between gap-2 rounded-md bg-muted/40 p-3">
            <span className="flex items-center gap-2 text-xs text-muted-foreground"><CheckCircle2 className="size-4 text-success" /> Último handshake exitoso: hoy 11:42</span>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => toast.success("Conexión exitosa con SUNAT (ping 238 ms)")}><RefreshCw data-icon="inline-start" /> Probar conexión</Button>
              <Button onClick={() => toast.success("Configuración de conexión guardada")}><Save data-icon="inline-start" /> Guardar</Button>
            </div>
          </div>
        </SectionCard>
      </TabsContent>

      <TabsContent value="series">
        <SectionCard title="Series fiscales por sede" action={<Button size="sm"><PlusCircle data-icon="inline-start" /> Nueva serie</Button>} contentClassName="p-0">
          <DataTable columns={[{ key: "type", header: "Tipo", cell: (s) => <DocumentTypeBadge type={/factura/i.test(s.type) ? "factura" : /boleta/i.test(s.type) ? "boleta" : /cr[eé]dito/i.test(s.type) ? "nota_credito" : /gu[ií]a/i.test(s.type) ? "guia_remision" : "proforma"} /> }, { key: "series", header: "Serie", cell: (s) => <span className="font-mono text-sm font-bold">{s.series}</span> }, { key: "site", header: "Sede", cell: (s) => s.site }, { key: "last", header: "Último emitido", align: "right", cell: (s) => <span className="font-mono font-semibold text-primary tabular-nums">{s.series}-{String(s.last).padStart(8, "0")}</span> }, { key: "actions", header: <span className="sr-only">Acción</span>, align: "right", cell: () => <Button variant="ghost" size="icon-sm" aria-label="Editar serie"><Pencil /></Button> }]} rows={config.series} rowKey={(s) => s.series} minWidth="640px" />
        </SectionCard>
      </TabsContent>

      <TabsContent value="impresion">
        <SectionCard title="Plantilla de impresión" contentClassName="grid gap-4 p-4 lg:grid-cols-2">
          <div className="flex min-w-0 flex-col gap-4">
            <Field label="Formato">
              <div className="grid grid-cols-2 gap-2">
                {[["a4", "Hoja A4"], ["ticket", "Ticket 80 mm"]].map(([v, l]) => (
                  <button key={v} type="button" onClick={() => setFormat(v)} aria-pressed={format === v} className={cn("rounded-md border px-3 py-2 text-sm font-medium", format === v && "border-primary bg-accent text-accent-foreground")}>{l}</button>
                ))}
              </div>
            </Field>
            <Field label="Leyenda al pie del comprobante"><Textarea rows={3} defaultValue={config.print.legend} /></Field>
            <Field label="Logo (PNG, máx. 500 KB)"><Input type="file" accept="image/png" /></Field>
            <Button className="w-fit" onClick={() => toast.success("Plantilla guardada")}><Save data-icon="inline-start" /> Guardar cambios</Button>
          </div>
          <div className={cn("mx-auto rounded border bg-white p-4 text-[10px] text-black shadow-sm", format === "a4" ? "aspect-[210/297] w-full max-w-72" : "w-56")}>
            <p className="font-bold">CLÍNICA DENTAL SONRISA S.A.C.</p>
            <p>RUC 20608941235 · Av. Larco 743, Miraflores</p>
            <p className="mt-2 border py-1 text-center font-bold">FACTURA ELECTRÓNICA F001-00000842</p>
            <p className="mt-2">Cliente: INVERSIONES ALPAMAYO S.A.C.</p>
            <div className="mt-2 border-t border-dashed pt-1">1 × Profilaxis LED premium ……… 448.40</div>
            <div>2 × Exodoncia simple ……………… 354.00</div>
            <div className="mt-2 text-right font-bold">TOTAL S/ 1,280.00</div>
            <p className="mt-3 text-[8px] text-neutral-500">{config.print.legend}</p>
          </div>
        </SectionCard>
      </TabsContent>
    </Tabs>
  );
}
