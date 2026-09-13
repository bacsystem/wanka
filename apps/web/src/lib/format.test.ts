import { describe, expect, it } from "vitest";
import {
  formatCurrency,
  formatDate,
  formatDocumentNumber,
  formatInteger,
  formatPercent,
  parseNumber,
} from "./format";

describe("formatCurrency", () => {
  it("formats soles with S/ prefix and two decimals", () => {
    expect(formatCurrency(4892.5)).toBe("S/ 4,892.50");
  });
  it("formats zero", () => {
    expect(formatCurrency(0)).toBe("S/ 0.00");
  });
  it("formats USD", () => {
    expect(formatCurrency(1200, "USD")).toBe("$ 1,200.00");
  });
});

describe("formatInteger", () => {
  it("adds thousands separators", () => {
    expect(formatInteger(12345)).toBe("12,345");
  });
});

describe("formatPercent", () => {
  it("adds sign and one decimal", () => {
    expect(formatPercent(14.2)).toBe("+14.2%");
    expect(formatPercent(-3)).toBe("-3.0%");
    expect(formatPercent(0)).toBe("0.0%");
  });
});

describe("formatDate", () => {
  it("formats ISO dates as dd/mm/yyyy", () => {
    expect(formatDate("2026-09-12")).toBe("12/09/2026");
  });
  it("formats ISO datetimes without shifting the calendar day", () => {
    expect(formatDate("2026-09-12T23:30:00")).toBe("12/09/2026");
  });
});

describe("formatDocumentNumber", () => {
  it("pads the correlative to 8 digits", () => {
    expect(formatDocumentNumber("F001", 1247)).toBe("F001-00001247");
  });
});

describe("parseNumber", () => {
  it("accepts dot and comma decimals", () => {
    expect(parseNumber("1.5")).toBe(1.5);
    expect(parseNumber("1,5")).toBe(1.5);
    expect(parseNumber(" 12 ")).toBe(12);
  });
  it("maps empty, invalid and negative input to 0", () => {
    expect(parseNumber("")).toBe(0);
    expect(parseNumber("abc")).toBe(0);
    expect(parseNumber("-3")).toBe(0);
  });
});
