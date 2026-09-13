export const pdt621 = {
  period: "2026-09", regime: "Régimen MYPE Tributario (RMT)", coefficient: "1.0% (hasta 300 UIT)",
  sales: 36313.56, debit: 6536.44, purchases: 24110, credit: 4339.83, carryOver: 0, igvToPay: 2196.61, rentaAdvance: 363.14,
  total: 2559.75, dueDigit: 5, dueDate: "2026-10-16", daysLeft: 34,
  alerts: ["2 compras registradas en RCE aún no figuran en la propuesta preliminar SUNAT (F002-003891 y E001-000412)."],
  cells: [
    { cell: "100", label: "Ventas netas gravadas", base: 36313.56, tax: 6536.44, source: "RVIE" },
    { cell: "106", label: "Ventas no gravadas (exoneradas)", base: 8, tax: 0, source: "RVIE" },
    { cell: "107", label: "Ventas inafectas", base: 0, tax: 0, source: "RVIE" },
    { cell: "140", label: "Total débito fiscal", base: 0, tax: 6536.44, source: "Cálculo" },
    { cell: "107b", label: "Compras netas gravadas destinadas a ventas gravadas", base: 24110, tax: 4339.83, source: "RCE" },
    { cell: "145", label: "Total crédito fiscal", base: 0, tax: 4339.83, source: "Cálculo" },
    { cell: "184", label: "Impuesto resultante (IGV a pagar)", base: 0, tax: 2196.61, source: "Cálculo" },
    { cell: "301", label: "Ingresos netos del mes (renta)", base: 36313.56, tax: 0, source: "RVIE" },
    { cell: "312", label: "Pago a cuenta renta (1.0%)", base: 0, tax: 363.14, source: "Cálculo" },
  ],
};

export const kardexConsolidated = [
  { warehouse: "Miraflores (principal)", rows: [
    { category: "Insumos clínicos", opening: 18200, inflow: 6400, outflow: 5900, closing: 18700 },
    { category: "Medicamentos", opening: 9800, inflow: 3200, outflow: 2900, closing: 10100 },
    { category: "Material dental", opening: 12100, inflow: 2100, outflow: 3350, closing: 10850 },
    { category: "Instrumental quirúrgico", opening: 8400, inflow: 0, outflow: 250, closing: 8150 },
  ] },
  { warehouse: "San Isidro (consultorios)", rows: [
    { category: "Insumos clínicos", opening: 6100, inflow: 2400, outflow: 2150, closing: 6350 },
    { category: "Medicamentos", opening: 3900, inflow: 1200, outflow: 980, closing: 4120 },
    { category: "Material dental", opening: 5200, inflow: 900, outflow: 1230, closing: 4870 },
  ] },
  { warehouse: "Surquillo (logístico)", rows: [
    { category: "Insumos clínicos", opening: 9600, inflow: 8200, outflow: 5300, closing: 12500 },
    { category: "Medicamentos", opening: 5400, inflow: 2600, outflow: 2100, closing: 5900 },
    { category: "Material dental", opening: 2700, inflow: 400, outflow: 409.5, closing: 2690.5 },
  ] },
];
