export interface Pet { id: string; name: string; species: string; breed: string; sex: string; age: string; birth: string; weightKg: number; chip: string; hc: string; ownerId: string; owner: string; ownerDoc: string; ownerPhone: string; alerts: string[]; lastVisit: string; visits: number; debt: number; weights: { label: string; kg: number }[] }
export interface Vaccine { name: string; appliedAt: string; lot: string; lab: string; vet: string; nextAt: string; status: "vigente" | "por_vencer" | "vencida" }
export interface VetEvolution { date: string; reason: string; vitals: { kg: number; temp: number; fc: number; fr: number }; diagnosis: string; plan: string; vet: string; amount: number }

export const petsMock: Pet[] = [
  { id: "p1", name: "Rocky", species: "Canino", breed: "Golden Retriever", sex: "Macho castrado", age: "4 años", birth: "2022-04-08", weightKg: 32, chip: "985112004567890", hc: "HC-VET-2023-0418", ownerId: "u1", owner: "Juan Carlos Pérez Huamán", ownerDoc: "DNI 45892147", ownerPhone: "+51 987 654 321", alerts: ["Alergia severa a AINEs (ketoprofeno / meloxicam)", "Agresivo por dolor en consulta: usar bozal canino L"], lastVisit: "2026-09-02", visits: 14, debt: 0, weights: [{ label: "Mar", kg: 30.1 }, { label: "May", kg: 31.2 }, { label: "Jul", kg: 31.5 }, { label: "Ago", kg: 31.8 }, { label: "Hoy", kg: 32 }] },
  { id: "p2", name: "Luna", species: "Felino", breed: "Siamés", sex: "Hembra", age: "2 años", birth: "2024-02-14", weightKg: 4.2, chip: "985112004511223", hc: "HC-VET-2024-0102", ownerId: "u2", owner: "María Castillo Quispe", ownerDoc: "DNI 72109843", ownerPhone: "+51 987 111 222", alerts: [], lastVisit: "2026-08-20", visits: 6, debt: 120, weights: [{ label: "Mar", kg: 3.9 }, { label: "May", kg: 4.0 }, { label: "Jul", kg: 4.1 }, { label: "Ago", kg: 4.2 }, { label: "Hoy", kg: 4.2 }] },
  { id: "p3", name: "Max", species: "Canino", breed: "Bulldog francés", sex: "Macho", age: "6 años", birth: "2020-06-01", weightKg: 13, chip: "—", hc: "HC-VET-2021-0090", ownerId: "u5", owner: "Jorge Rojas Barrenechea", ownerDoc: "DNI 10482910", ownerPhone: "+51 999 888 777", alerts: ["Braquicéfalo: precaución con sedación"], lastVisit: "2026-09-10", visits: 22, debt: 0, weights: [{ label: "Mar", kg: 12.4 }, { label: "May", kg: 12.8 }, { label: "Jul", kg: 13.1 }, { label: "Ago", kg: 13 }, { label: "Hoy", kg: 13 }] },
];

export const vaccinesMock: Vaccine[] = [
  { name: "Séxtuple (DHPPi+L)", appliedAt: "2026-04-10", lot: "SX-88213", lab: "Zoetis", vet: "M.V. Rodrigo Morales", nextAt: "2027-04-10", status: "vigente" },
  { name: "Antirrábica", appliedAt: "2026-04-10", lot: "RB-10021", lab: "MSD", vet: "M.V. Rodrigo Morales", nextAt: "2027-04-10", status: "vigente" },
  { name: "Bordetella KC (Bronchicine)", appliedAt: "2025-09-30", lot: "KC-5501", lab: "Zoetis", vet: "M.V. Ana Torres", nextAt: "2026-09-30", status: "por_vencer" },
  { name: "Desparasitación interna (Drontal Plus)", appliedAt: "2026-06-15", lot: "DP-3391", lab: "Elanco", vet: "M.V. Rodrigo Morales", nextAt: "2026-09-15", status: "vencida" },
  { name: "Desparasitación externa (Bravecto)", appliedAt: "2026-07-01", lot: "BV-7720", lab: "MSD", vet: "M.V. Ana Torres", nextAt: "2026-10-01", status: "vigente" },
];

export const vetEvolutions: VetEvolution[] = [
  { date: "2026-09-02", reason: "Dermatitis atópica · prurito en flancos", vitals: { kg: 32, temp: 38.6, fc: 88, fr: 22 }, diagnosis: "Dermatitis atópica canina (L20)", plan: "Baño medicado semanal, Apoquel 16 mg c/12 h por 14 días. Control en 15 días.", vet: "M.V. Rodrigo Morales", amount: 180 },
  { date: "2026-07-14", reason: "Control anual y peso", vitals: { kg: 31.5, temp: 38.4, fc: 84, fr: 20 }, diagnosis: "Paciente sano · sobrepeso leve", plan: "Dieta light 320 g/día, ejercicio 40 min.", vet: "M.V. Ana Torres", amount: 60 },
  { date: "2026-04-10", reason: "Vacunación anual", vitals: { kg: 31.2, temp: 38.5, fc: 90, fr: 22 }, diagnosis: "Inmunización", plan: "Séxtuple + antirrábica. Próxima 04/2027.", vet: "M.V. Rodrigo Morales", amount: 140 },
];

export const groomingMock = [{ date: "2026-08-22", service: "Baño y corte higiénico", groomer: "Karla", amount: 65, notes: "Sin incidencias" }, { date: "2026-06-30", service: "Baño medicado", groomer: "Karla", amount: 55, notes: "Shampoo clorhexidina" }];
