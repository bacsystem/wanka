export type ToothState = "sano" | "caries" | "resina" | "corona" | "extraer" | "ausente" | "implante" | "endodoncia";

export const toothStateMeta: Record<ToothState, { label: string; short: string; className: string }> = {
  sano: { label: "Sano", short: "S", className: "bg-card border-border" },
  caries: { label: "Caries activa", short: "CAR", className: "bg-destructive/15 border-destructive text-destructive" },
  resina: { label: "Resina", short: "RES", className: "bg-primary/15 border-primary text-primary" },
  corona: { label: "Corona", short: "COR", className: "bg-warning/15 border-warning text-warning" },
  extraer: { label: "Por extraer", short: "EXT", className: "bg-destructive/10 border-destructive border-dashed text-destructive" },
  ausente: { label: "Ausente", short: "AUS", className: "bg-muted border-transparent text-muted-foreground line-through" },
  implante: { label: "Implante", short: "IMP", className: "bg-accent border-primary/60 text-accent-foreground" },
  endodoncia: { label: "Endodoncia", short: "END", className: "bg-violet-500/15 border-violet-500 text-violet-600" },
};

// FDI numbering: quadrants 1–4 (permanent), each with 8 teeth.
export const upperRight = [18, 17, 16, 15, 14, 13, 12, 11];
export const upperLeft = [21, 22, 23, 24, 25, 26, 27, 28];
export const lowerLeft = [31, 32, 33, 34, 35, 36, 37, 38];
export const lowerRight = [48, 47, 46, 45, 44, 43, 42, 41];

export const initialOdontogram: Record<number, ToothState> = {
  18: "extraer", 16: "corona", 24: "resina", 28: "extraer", 36: "caries", 46: "implante", 48: "ausente", 26: "endodoncia",
};

export const toothNames: Record<number, string> = { 36: "Primer molar inferior izquierdo", 16: "Primer molar superior derecho", 46: "Primer molar inferior derecho", 18: "Tercer molar superior derecho", 28: "Tercer molar superior izquierdo", 24: "Primer premolar superior izquierdo", 26: "Primer molar superior izquierdo", 48: "Tercer molar inferior derecho" };

export interface Evolution { date: string; tooth?: number; procedure: string; notes: string; professional: string; document: string; amount: number; billed: boolean }

export const evolutions: Evolution[] = [
  { date: "2026-08-10", tooth: 16, procedure: "Cementación definitiva corona metal-porcelana", notes: "Sobre pilar biológico con ionómero de vidrio. Control oclusal sin interferencias.", professional: "Dr. Manuel Arévalo", document: "Factura F001-002391 (SUNAT OK)", amount: 420, billed: true },
  { date: "2026-06-14", procedure: "Profilaxis completa", notes: "Destartraje ultrasónico supragingival y pulido coronario con pasta profiláctica.", professional: "Dra. Claudia Ramos", document: "Boleta B001-008129", amount: 150, billed: true },
  { date: "2026-03-02", tooth: 24, procedure: "Restauración con resina", notes: "Clase II ocluso-distal, resina nanohíbrida A2.", professional: "Dra. Claudia Ramos", document: "Boleta B001-000305", amount: 180, billed: true },
];

export const treatmentPlan = [
  { step: 1, procedure: "Restauración resina 3M Z350 XT · pieza 3.6", session: "Sesión 1", priority: "Alta", cost: 180, status: "Programado" },
  { step: 2, procedure: "Exodoncia pieza 1.8", session: "Sesión 2", priority: "Media", cost: 150, status: "Pendiente" },
  { step: 3, procedure: "Exodoncia pieza 2.8", session: "Sesión 2", priority: "Media", cost: 150, status: "Pendiente" },
  { step: 4, procedure: "Corona zirconio · pieza 4.6 (implante)", session: "Sesión 3–4", priority: "Baja", cost: 1200, status: "Pendiente" },
  { step: 5, procedure: "Control y pulido estético", session: "Sesión 5", priority: "Baja", cost: 0, status: "Pendiente" },
];

export const radiographs = [{ name: "Periapical 3.6", date: "2026-09-12", kind: "RVG" }, { name: "Panorámica 2D", date: "2026-08-10", kind: "OPG" }];
