import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SunatStatusBadge } from "@/features/documents/components/sunat-status-badge";

describe("SunatStatusBadge", () => {
  it.each([
    ["aceptado", "Aceptado"],
    ["pendiente", "Pendiente"],
    ["rechazado", "Rechazado"],
  ] as const)("renders %s as %s", (status, label) => {
    render(<SunatStatusBadge status={status} />);
    const badge = screen.getByText(label);
    expect(badge).toBeInTheDocument();
    expect(badge.closest("[data-status]")).toHaveAttribute("data-status", status);
  });
});
