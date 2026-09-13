const CURRENCY_SYMBOL: Record<"PEN" | "USD", string> = {
  PEN: "S/",
  USD: "$",
};

const numberFormatter = new Intl.NumberFormat("en-US", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const integerFormatter = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

export function formatCurrency(value: number, currency: "PEN" | "USD" = "PEN") {
  return `${CURRENCY_SYMBOL[currency]} ${numberFormatter.format(value)}`;
}

export function formatInteger(value: number) {
  return integerFormatter.format(value);
}

export function formatPercent(value: number) {
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(1)}%`;
}

/** dd/mm/yyyy from an ISO date or datetime string, using the calendar day as written. */
export function formatDate(iso: string) {
  const [year, month, day] = iso.slice(0, 10).split("-");
  return `${day}/${month}/${year}`;
}

export function formatDocumentNumber(series: string, number: number) {
  return `${series}-${String(number).padStart(8, "0")}`;
}

/** Numeric value typed in a quantity/price field: accepts "1,5" or "1.5", never negative, empty → 0. */
export function parseNumber(value: string) {
  const n = Number(value.trim().replace(",", "."));
  return Number.isFinite(n) && n > 0 ? n : 0;
}
