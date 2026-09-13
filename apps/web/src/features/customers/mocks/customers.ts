import type { Customer } from "@/types/domain";

export const customersMock: Customer[] = [
  { id: "u1", documentType: "DNI", documentNumber: "45892147", name: "Juan Carlos Pérez Huamán", phone: "+51 987 654 321", email: "jc.perez@gmail.com", segment: "Paciente frecuente", lastOperation: "2026-09-11", billedTotal: 2140, fiscalStatus: "al_dia", clinicalRecord: "HC-2023-014" },
  { id: "u2", documentType: "DNI", documentNumber: "72109843", name: "María Castillo Quispe", phone: "+51 987 111 222", email: "m.castillo@gmail.com", segment: "Paciente frecuente", lastOperation: "2026-09-10", billedTotal: 1450, fiscalStatus: "al_dia", clinicalRecord: "HC-2024-0392" },
  { id: "u3", documentType: "RUC", documentNumber: "20554182910", name: "Inversiones Alpamayo S.A.C.", phone: "+51 (01) 442-8990", email: "facturacion@alpamayo.pe", segment: "Convenio corporativo", lastOperation: "2026-09-09", billedTotal: 18620, fiscalStatus: "con_deuda" },
  { id: "u4", documentType: "RUC", documentNumber: "20601829411", name: "Servicios Logísticos Lima S.A.C.", phone: "+51 (01) 442-1200", email: "compras@serloglima.pe", segment: "Convenio ocupacional", lastOperation: "2026-09-02", billedTotal: 9400, fiscalStatus: "con_deuda" },
  { id: "u5", documentType: "DNI", documentNumber: "10482910", name: "Jorge Rojas Barrenechea", phone: "+51 999 888 777", segment: "Convenio Rimac EPS", lastOperation: "2026-08-28", billedTotal: 620, fiscalStatus: "al_dia", clinicalRecord: "HC-2022-118" },
  { id: "u6", documentType: "DNI", documentNumber: "41234567", name: "Ana Lucía Paredes Vega", phone: "+51 955 444 333", email: "ana.paredes@outlook.com", segment: "Convenio Pacífico EPS", lastOperation: "2026-07-15", billedTotal: 3180, fiscalStatus: "al_dia", clinicalRecord: "HC-2021-077" },
  { id: "u7", documentType: "CE", documentNumber: "001234567", name: "Giuseppe Moretti Rossi", phone: "+51 933 222 111", segment: "Paciente", lastOperation: "2026-05-03", billedTotal: 240, fiscalStatus: "inactivo", clinicalRecord: "HC-2025-201" },
  { id: "u8", documentType: "DNI", documentNumber: "08765432", name: "Carlos Raúl Benavides S.", phone: "+51 944 555 666", segment: "Paciente", lastOperation: "2026-09-08", billedTotal: 890, fiscalStatus: "al_dia", clinicalRecord: "HC-2020-009" },
];

export const genericCustomer: Customer = {
  id: "generic", documentType: "DNI", documentNumber: "00000000", name: "Cliente genérico", billedTotal: 0, fiscalStatus: "al_dia",
};
