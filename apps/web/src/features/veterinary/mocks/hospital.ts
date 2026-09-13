export type KennelStatus = "ocupado" | "libre" | "limpieza";
export interface Kennel { id: string; area: "general" | "uci" | "aislamiento"; status: KennelStatus; patient?: { petId: string; name: string; breed: string; kg: number; owner: string; ownerDoc: string; reason: string; day: number; ofDays: number; vet: string; nextMed: string; alert?: string; critical?: boolean } }
export interface Vital { time: string; temp: number; fc: number; fr: number; kg: number; mucosa: string; tllc: string; gluc: number; note: string }
export interface Medication { id: string; route: string; drug: string; dose: string; time: string; shift: string; done: boolean }
export interface GroomingJob { id: string; time: string; pet: string; breed: string; owner: string; phone: string; services: string[]; minutes: number; groomer: string; status: "en_espera" | "en_proceso" | "listo" | "entregado" }

export const hospitalSummary = { capacity: 12, occupied: 8, free: 2, cleaning: 2 };
export const kennels: Kennel[] = [
  { id: "K-01", area: "general", status: "ocupado", patient: { petId: "p1", name: "Rocky", breed: "Golden Retriever · macho castrado", kg: 32.4, owner: "Juan Carlos Pérez H.", ownerDoc: "DNI 45892147", reason: "Gastroenteritis hemorrágica · deshidratación 8%", day: 2, ofDays: 3, vet: "M.V. Rodrigo Morales", nextMed: "14:00 (metronidazol)", alert: "Agresivo por dolor · alergia a meloxicam" } },
  { id: "K-02", area: "general", status: "ocupado", patient: { petId: "p3", name: "Milo", breed: "Beagle · macho", kg: 14.2, owner: "Ana Sofía Morales", ownerDoc: "DNI 70821943", reason: "Post-quirúrgico: enterotomía por cuerpo extraño", day: 1, ofDays: 2, vet: "M.V. Claudia Mendoza", nextMed: "15:30 (tramadol)" } },
  { id: "UCI-01", area: "uci", status: "ocupado", patient: { petId: "p2", name: "Luna", breed: "Siamés · hembra", kg: 4.1, owner: "Carlos Rivas", ownerDoc: "DNI 10238472", reason: "Obstrucción urinaria FLUTD · sondaje uretral", day: 3, ofDays: 4, vet: "M.V. Rodrigo Morales", nextMed: "16:00 (lavado vesical)", critical: true } },
  { id: "K-04", area: "general", status: "limpieza" },
  { id: "K-05", area: "general", status: "libre" },
  { id: "K-06", area: "general", status: "libre" },
  { id: "K-07", area: "general", status: "ocupado", patient: { petId: "p4", name: "Toby", breed: "Schnauzer · macho", kg: 9.8, owner: "Lucía Fernández", ownerDoc: "DNI 44120987", reason: "Pancreatitis aguda", day: 1, ofDays: 3, vet: "M.V. Ana Torres", nextMed: "13:00 (maropitant)" } },
  { id: "K-08", area: "general", status: "ocupado", patient: { petId: "p5", name: "Nala", breed: "Mestizo · hembra", kg: 18, owner: "Pedro Salas", ownerDoc: "DNI 70998812", reason: "Observación post-ovariohisterectomía", day: 1, ofDays: 1, vet: "M.V. Claudia Mendoza", nextMed: "Alta 17:00" } },
  { id: "UCI-02", area: "uci", status: "limpieza" },
  { id: "AIS-01", area: "aislamiento", status: "ocupado", patient: { petId: "p6", name: "Simba", breed: "Persa · macho", kg: 5.2, owner: "Rosa Villanueva", ownerDoc: "DNI 09876543", reason: "Panleucopenia felina · aislamiento", day: 4, ofDays: 7, vet: "M.V. Ana Torres", nextMed: "14:30 (fluidoterapia)", critical: true } },
  { id: "AIS-02", area: "aislamiento", status: "ocupado", patient: { petId: "p7", name: "Coco", breed: "Cocker · macho", kg: 12, owner: "Miguel Guerrero", ownerDoc: "DNI 70192841", reason: "Parvovirosis · día 5", day: 5, ofDays: 6, vet: "M.V. Rodrigo Morales", nextMed: "15:00 (ondansetrón)" } },
  { id: "K-03", area: "general", status: "ocupado", patient: { petId: "p8", name: "Kira", breed: "Husky · hembra", kg: 22, owner: "Karla Huamán", ownerDoc: "DNI 70998813", reason: "Fractura de radio · reposo post-osteosíntesis", day: 2, ofDays: 2, vet: "M.V. Claudia Mendoza", nextMed: "16:30 (meloxicam)" } },
];
export const vitalsMock: Vital[] = [
  { time: "12:00", temp: 38.6, fc: 105, fr: 24, kg: 32.4, mucosa: "Rosadas", tllc: "< 2 s", gluc: 94, note: "Alerta, sin vómitos (M.V. Rodrigo)" },
  { time: "08:00", temp: 38.9, fc: 118, fr: 28, kg: 32.0, mucosa: "Pálidas +", tllc: "2 s", gluc: 88, note: "Leve decaimiento (Téc. Sandra)" },
  { time: "04:00", temp: 39.4, fc: 130, fr: 32, kg: 31.8, mucosa: "Secas", tllc: "3 s", gluc: 102, note: "Pico febril nocturno (Dr. Mendoza)" },
];
export const medsMock: Medication[] = [
  { id: "m1", route: "EV", drug: "Metronidazol 500 mg / 100 mL", dose: "15 mg/kg (9.7 mL) c/12 h · infusión 20 min", time: "14:00", shift: "Turno tarde · M.V. Morales", done: false },
  { id: "m2", route: "SC", drug: "Maropitant 10 mg/mL", dose: "1 mg/kg (3.2 mL) c/24 h", time: "09:00", shift: "Turno mañana · Téc. Sandra", done: true },
  { id: "m3", route: "EV", drug: "Ringer lactato", dose: "Fluidoterapia 60 mL/kg/día · 80 mL/h", time: "Continua", shift: "Bomba de infusión 2", done: true },
  { id: "m4", route: "VO", drug: "Dieta gastrointestinal húmeda", dose: "60 g c/6 h · tolerancia", time: "12:00", shift: "Téc. Sandra", done: true },
];
export const groomingJobs: GroomingJob[] = [
  { id: "g1", time: "09:00", pet: "Bruno", breed: "Labrador", owner: "Diana Solís", phone: "+51 966 111 222", services: ["Baño", "Corte de uñas"], minutes: 60, groomer: "Karla", status: "entregado" },
  { id: "g2", time: "10:30", pet: "Mía", breed: "Poodle", owner: "Renato Cano", phone: "+51 955 333 444", services: ["Baño", "Corte", "Oídos"], minutes: 90, groomer: "Karla", status: "listo" },
  { id: "g3", time: "11:00", pet: "Zeus", breed: "Pastor alemán", owner: "Andrea Benavides", phone: "+51 944 555 666", services: ["Baño medicado"], minutes: 75, groomer: "Pedro", status: "en_proceso" },
  { id: "g4", time: "12:30", pet: "Coco", breed: "Shih tzu", owner: "Elena Ruiz", phone: "+51 933 777 888", services: ["Baño", "Corte", "Uñas", "Oídos"], minutes: 90, groomer: "Karla", status: "en_espera" },
  { id: "g5", time: "14:00", pet: "Lola", breed: "Gato persa", owner: "Fernando Zegarra", phone: "+51 922 999 000", services: ["Baño seco", "Cepillado"], minutes: 45, groomer: "Pedro", status: "en_espera" },
];
