import { BadgeCheck, Bolt, Boxes, Layers, ShieldCheck, Star, Table2 } from "lucide-react";

const features = [
  { icon: Bolt, title: "Boletas y facturas en segundos", text: "Validación en línea con OSE autorizado y CDR inmediato." },
  { icon: Boxes, title: "Inventario multisede y kardex 13.1", text: "Control de insumos por lote, caducidad y traslados GRE." },
  { icon: Table2, title: "Reportes SIRE y sincronización", text: "RVIE/RCE con libros electrónicos y exportación a Concar." },
];

export function BrandPanel() {
  return (
    <aside
      className="border-borde-tarjeta bg-card text-foreground shadow-tarjeta relative hidden flex-col justify-between overflow-hidden rounded-xl border p-10 lg:flex"
      aria-label="Acerca de Wanka"
    >
      <div>
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <span className="flex size-10 items-center justify-center rounded-xl bg-acento-superficie text-acento-superficie-texto">
              <Layers className="size-5" />
            </span>
            <div>
              <p className="flex items-center gap-2 text-lg font-bold leading-tight tracking-tight">
                Wanka{" "}
                <span className="rounded-full bg-acento-suave px-2 py-0.5 text-[10px] font-semibold tracking-wide text-acento-superficie-texto">
                  CLOUD
                </span>
              </p>
              <p className="text-etiqueta text-muted-foreground">Sistemas de gestión y facturación SUNAT</p>
            </div>
          </div>
        </div>

        <span className="mt-10 inline-flex items-center gap-1.5 rounded-full bg-acento-suave px-3 py-1 text-etiqueta font-semibold text-acento-superficie-texto">
          <BadgeCheck className="size-3.5" /> Especializado en salud y servicios MYPE
        </span>
        <h1 className="mt-3 max-w-md text-3xl font-bold tracking-tight">
          Gestión clínica, control de caja y emisión tributaria sin fisuras.
        </h1>
        <p className="mt-3 max-w-md text-cuerpo text-muted-foreground">
          Plataforma autorizada para clínicas, veterinarias, restaurantes y comercios con emisión directa de comprobantes electrónicos.
        </p>

        <ul className="mt-8 flex flex-col gap-3">
          {features.map((f) => (
            <li key={f.title} className="flex gap-3 rounded-xl bg-secondary p-3.5">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-acento-suave text-acento-superficie-texto">
                <f.icon className="size-4" strokeWidth={2} />
              </span>
              <div>
                <p className="text-control font-semibold text-foreground">{f.title}</p>
                <p className="text-etiqueta text-muted-foreground">{f.text}</p>
              </div>
            </li>
          ))}
        </ul>
      </div>

      <blockquote className="rounded-xl bg-secondary p-4 text-cuerpo">
        <div className="flex items-start gap-3">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-acento-superficie text-xs font-bold text-acento-superficie-texto">
            CD
          </span>
          <div className="min-w-0 flex-1">
            <p className="italic text-foreground">“Emitimos más de 180 comprobantes diarios sin caídas del sistema.”</p>
            <footer className="mt-1 flex items-center justify-between text-etiqueta text-muted-foreground">
              <span>Dra. Claudia Ramos · Administradora</span>
              <span className="flex items-center gap-1 font-semibold text-foreground">
                <Star className="size-3 fill-current" /> 5.0
              </span>
            </footer>
          </div>
        </div>
        <p className="mt-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-t border-border/60 pt-3 text-[11px] text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <ShieldCheck className="size-3" /> ISO/IEC 27001
          </span>
          <span className="inline-flex items-center gap-1">
            <BadgeCheck className="size-3" /> Homologado SUNAT
          </span>
          <span>Lima, Perú</span>
        </p>
      </blockquote>
    </aside>
  );
}
