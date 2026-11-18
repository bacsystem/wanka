import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DataTable, type Column } from "./data-table";

interface Row { id: string; name: string; total: number }
const rows: Row[] = [
  { id: "1", name: "José Pérez", total: 10 },
  { id: "2", name: "Ana Ruiz", total: 20 },
];
const columns: Column<Row>[] = [
  { key: "name", header: "Cliente", cell: (r) => <strong>{r.name}</strong> },
  { key: "total", header: "Total", align: "right", cell: (r) => r.total },
];

describe("DataTable (DS adapter)", () => {
  it("searches the visible text of rendered cells, ignoring accents", () => {
    render(<DataTable columns={columns} rows={rows} rowKey={(r) => r.id} />);
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "jose" } });
    expect(screen.getByText("José Pérez")).toBeTruthy();
    expect(screen.queryByText("Ana Ruiz")).toBeNull();
  });
  it("renders the empty message when there are no rows", () => {
    render(<DataTable columns={columns} rows={[]} rowKey={(r) => r.id} emptyMessage="Nada por aquí" />);
    expect(screen.getByText("Nada por aquí")).toBeTruthy();
  });
  it("drops the bar with controls={false}", () => {
    render(<DataTable controls={false} columns={columns} rows={rows} rowKey={(r) => r.id} />);
    expect(screen.queryByRole("searchbox")).toBeNull();
    expect(screen.getByText("Ana Ruiz")).toBeTruthy();
  });
});
