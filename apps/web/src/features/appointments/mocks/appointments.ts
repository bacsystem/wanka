export type AppointmentStatus = "atendido" | "en_silla" | "en_espera" | "confirmado" | "agendado" | "cancelado";

export interface Chair {
  id: string;
  label: string;
  professional: string;
}

export interface Appointment {
  id: string;
  code: string;
  chairId: string;
  start: string; // HH:mm
  end: string;
  patient: string;
  document: string;
  age?: number;
  procedure: string;
  professional: string;
  status: AppointmentStatus;
  price?: number;
  phone?: string;
  note?: string;
}

export const chairs: Chair[] = [
  { id: "s1", label: "Sillón 1", professional: "Dra. Mendoza" },
  { id: "s2", label: "Sillón 2", professional: "Dr. Flores" },
  { id: "s3", label: "Sillón 3", professional: "Higiene y profilaxis" },
];

export const agendaSummary = { today: 28, waiting: 3, readyToBill: 14, cancelled: 2, date: "2026-09-12" };

export const appointmentsMock: Appointment[] = [
  { id: "a1", code: "OD-8492", chairId: "s1", start: "08:30", end: "09:15", patient: "Juan Carlos Pérez Huamán", document: "DNI 45892147", age: 38, procedure: "Curación resina 3M pieza 3.6", professional: "Dra. Mendoza", status: "atendido", price: 240, phone: "+51 987 654 321", note: "Procedimiento ejecutado · listo para facturar en POS" },
  { id: "a2", code: "OD-8493", chairId: "s2", start: "08:30", end: "09:00", patient: "Mantenimiento de turbina", document: "—", procedure: "Bloqueo técnico", professional: "Dr. Flores", status: "cancelado" },
  { id: "a3", code: "OD-8494", chairId: "s1", start: "09:30", end: "10:30", patient: "Carlos Rivas Salazar", document: "DNI 45892104", age: 29, procedure: "Control mensual de brackets", professional: "Dra. Mendoza", status: "en_silla", price: 120, phone: "+51 944 111 222" },
  { id: "a4", code: "OD-8495", chairId: "s3", start: "09:30", end: "10:00", patient: "Ana Lucía Paredes Vega", document: "DNI 41234567", age: 41, procedure: "Profilaxis ultrasonido", professional: "Higienista R. Cano", status: "atendido", price: 90 },
  { id: "a5", code: "OD-8496", chairId: "s2", start: "10:15", end: "11:15", patient: "Mariana Quispe Flores", document: "DNI 72109843", age: 27, procedure: "Profilaxis + fluorización", professional: "Dr. Flores", status: "en_espera", price: 90, phone: "+51 987 111 222", note: "En sala de espera hace 8 min" },
  { id: "a6", code: "OD-8497", chairId: "s1", start: "11:30", end: "12:30", patient: "Inv. Alpamayo S.A.C. (3 colaboradores)", document: "RUC 20554182910", procedure: "Evaluación ocupacional", professional: "Dra. Mendoza", status: "confirmado", price: 450 },
  { id: "a7", code: "OD-8498", chairId: "s3", start: "11:00", end: "11:30", patient: "Giuseppe Moretti Rossi", document: "CE 001234567", age: 52, procedure: "Profilaxis simple", professional: "Higienista R. Cano", status: "agendado", price: 70 },
  { id: "a8", code: "OD-8499", chairId: "s2", start: "12:00", end: "12:45", patient: "Jorge Rojas Barrenechea", document: "DNI 10482910", age: 45, procedure: "Radiografía + diagnóstico", professional: "Dr. Flores", status: "agendado", price: 60 },
  { id: "a9", code: "OD-8500", chairId: "s1", start: "14:00", end: "15:00", patient: "Renzo Valdivia Ortiz", document: "DNI 70112233", age: 33, procedure: "Extracción pieza 4.8", professional: "Dra. Mendoza", status: "agendado", price: 150 },
  { id: "a10", code: "OD-8501", chairId: "s3", start: "14:30", end: "15:00", patient: "María Castillo Quispe", document: "DNI 72109843", age: 28, procedure: "Profilaxis", professional: "Higienista R. Cano", status: "confirmado", price: 70 },
  { id: "a11", code: "OD-8502", chairId: "s2", start: "15:00", end: "16:00", patient: "Carlos Raúl Benavides S.", document: "DNI 08765432", age: 61, procedure: "Prótesis: prueba de estructura", professional: "Dr. Flores", status: "agendado", price: 0 },
];

export const hours = ["08:00", "09:00", "10:00", "11:00", "12:00", "13:00", "14:00", "15:00", "16:00"];
