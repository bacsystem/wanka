import { render, screen } from "@testing-library/react";
import { Wallet } from "lucide-react";
import { describe, expect, it } from "vitest";
import { KpiCard } from "./kpi-card";

describe("KpiCard", () => {
  it("renders a currency KPI with positive delta", () => {
    render(
      <KpiCard
        icon={Wallet}
        kpi={{ id: "a", label: "Ventas de hoy", value: 4892.5, format: "currency", deltaPercent: 14.2 }}
      />,
    );
    expect(screen.getByTestId("kpi-value")).toHaveTextContent("S/ 4,892.50");
    expect(screen.getByText("+14.2%")).toBeInTheDocument();
  });

  it("renders an integer KPI without delta", () => {
    render(
      <KpiCard
        icon={Wallet}
        kpi={{ id: "b", label: "Stock bajo", value: 5, format: "integer", deltaPercent: null, hint: "2 agotados" }}
      />,
    );
    expect(screen.getByTestId("kpi-value")).toHaveTextContent("5");
    expect(screen.queryByText(/%$/)).not.toBeInTheDocument();
    expect(screen.getByText("2 agotados")).toBeInTheDocument();
  });
});
