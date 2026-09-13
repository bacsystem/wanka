export interface ReceptionLine { id: string; sku: string; name: string; lot: string; expires: string; sent: number; received: number; location: string; note?: string }

export const receptionMock = {
  guide: "T001-000428",
  status: "En tránsito · arribado a almacén",
  origin: "Sede Miraflores (principal)",
  destination: "Sede San Isidro (subalmacén)",
  carrier: { name: "Logística Express Dental S.A.C.", ruc: "20556677881" },
  driver: { name: "Luis Huamán", plate: "B8F-912", license: "M1-C2" },
  dispatchedAt: "2026-09-12",
  lines: [
    { id: "r1", sku: "MAT-0104", name: "Resina nanohíbrida 3M Filtek Z350 A2", lot: "L-9024", expires: "11/2027", sent: 20, received: 20, location: "Vitrina odontología 1" },
    { id: "r2", sku: "INSU-0055", name: "Agujas dentales desechables 27G cortas (x100)", lot: "AG-3301", expires: "05/2028", sent: 10, received: 9, location: "Estante A-02", note: "Faltante en bulto" },
    { id: "r3", sku: "ANEST-002", name: "Anestesia lidocaína 2% con epinefrina (x50)", lot: "LID-8820", expires: "08/2027", sent: 5, received: 5, location: "Gabinete frío", note: "1 ampolla fisurada en transporte" },
    { id: "r4", sku: "PROT-ZIR-44", name: "Corona zirconio fresada multicapa", lot: "L-CAD-102", expires: "—", sent: 2, received: 2, location: "Bandeja prótesis" },
  ] as ReceptionLine[],
  locations: ["Vitrina odontología 1", "Estante A-02", "Gaveta quirúrgica B", "Gabinete frío", "Vitrina B-01", "Bandeja prótesis", "Gaveta cirugía"],
};

export interface CountLine { id: string; sku: string; name: string; presentation: string; system: number; counted: number; unitCost: number }

export const physicalCountMock: CountLine[] = [
  { id: "c1", sku: "MAT-0104", name: "Resina nanohíbrida 3M Filtek Z350 A2", presentation: "Jeringa 4 g · lote L-9024", system: 42, counted: 42, unitCost: 95 },
  { id: "c2", sku: "INSU-0055", name: "Agujas dentales desechables 27G cortas", presentation: "Caja x 100 · lote AG-3301", system: 18, counted: 16, unitCost: 32 },
  { id: "c3", sku: "MAT-0310", name: "Adhesivo dental Single Bond Universal 5 ml", presentation: "Frasco dosificador · vence 03/2027", system: 6, counted: 5, unitCost: 120 },
  { id: "c4", sku: "MAT-0881", name: "Acrílico dental autopolimerizable Marche 60 g", presentation: "Polímero rosa jaspeado", system: 15, counted: 15, unitCost: 24.5 },
  { id: "c5", sku: "MED-0441", name: "Lidocaína 2% c/epinefrina", presentation: "Caja 50 carpules · lote LT-98214", system: 34, counted: 34, unitCost: 78.5 },
];
