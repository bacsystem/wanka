import { BadgeCheck, Bolt, Boxes, Layers, ShieldCheck, Star, Table2 } from "lucide-react";

const features = [
  { icon: Bolt, title: "Boletas y facturas en segundos", text: "Validación en línea con OSE autorizado y CDR inmediato." },
  { icon: Boxes, title: "Inventario multisede y kardex 13.1", text: "Control de insumos por lote, caducidad y traslados GRE." },
  { icon: Table2, title: "Reportes SIRE y sincronización", text: "RVIE/RCE con libros electrónicos y exportación a Concar." },
];

export function BrandPanel() {
  return (
    <aside className="relative hidden flex-col justify-between overflow-hidden bg-[#0b3d3a] p-10 text-white lg:flex" aria-label="Acerca de Wanka">
      <div className="pointer-events-none absolute -top-24 -right-24 size-80 rounded-full bg-primary/30 blur-3xl" />
      <div>
        <div className="flex items-center gap-2">
          <span className="flex size-10 items-center justify-center rounded-lg bg-white/15"><Layers className="size-5" /></span>
          <div><p className="flex items-center gap-2 text-xl font-bold leading-tight">Wanka <span className="rounded bg-white/15 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide">CLOUD</span></p><p className="text-xs text-white/70">Sistemas de gestión y facturación SUNAT</p></div>
        </div>
        <span className="mt-10 inline-flex items-center gap-1.5 rounded-full bg-white/10 px-3 py-1 text-xs font-medium"><BadgeCheck className="size-3.5" /> Especializado en salud y servicios MYPE</span>
        <h1 className="mt-3 max-w-md text-3xl font-bold tracking-tight">Gestión clínica, control de caja y emisión tributaria sin fisuras.</h1>
        <p className="mt-3 max-w-md text-sm text-white/75">Plataforma autorizada para clínicas, veterinarias, restaurantes y comercios con emisión directa de comprobantes electrónicos.</p>
        <ul className="mt-8 flex flex-col gap-4">
          {features.map((f) => (
            <li key={f.title} className="flex gap-3 rounded-xl bg-white/10 p-3">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-white/15"><f.icon className="size-4" strokeWidth={2} /></span>
              <div><p className="text-sm font-semibold">{f.title}</p><p className="text-xs text-white/70">{f.text}</p></div>
            </li>
          ))}
        </ul>
      </div>
      <div>
        <blockquote className="rounded-xl bg-white/10 p-4 text-sm">
          <div className="flex items-start gap-3"><span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-white/20 text-xs font-bold">CD</span><div className="min-w-0 flex-1"><p className="italic">“Emitimos más de 180 comprobantes diarios sin caídas del sistema.”</p><footer className="mt-1 flex items-center justify-between text-xs text-white/70"><span>Dra. Claudia Ramos · Administradora</span><span className="flex items-center gap-1"><Star className="size-3 fill-current" /> 5.0</span></footer></div></div>
          <p className="mt-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-t border-white/10 pt-3 text-[11px] text-white/60"><span className="inline-flex items-center gap-1"><ShieldCheck className="size-3" /> ISO/IEC 27001</span><span className="inline-flex items-center gap-1"><BadgeCheck className="size-3" /> Homologado SUNAT</span><span>Lima, Perú</span></p>
        </blockquote>
      </div>
    </aside>
  );
}
