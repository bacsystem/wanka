import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PaginationBar } from "./pagination";

describe("PaginationBar", () => {
  it("marks the current page after navigating past the first window", () => {
    render(<PaginationBar from={1} to={25} total={200} />);
    const next = screen.getByRole("button", { name: /Siguiente/ });
    fireEvent.click(next);
    fireEvent.click(next);
    fireEvent.click(next);
    expect(screen.getByText("4", { selector: "[aria-current=page]" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Siguiente/ })).toBeEnabled();
  });
  it("reports page changes when controlled", () => {
    const onPageChange = vi.fn();
    render(<PaginationBar from={26} to={50} total={200} page={2} onPageChange={onPageChange} />);
    fireEvent.click(screen.getByRole("button", { name: "8" }));
    expect(onPageChange).toHaveBeenCalledWith(8);
    expect(screen.getByText("2", { selector: "[aria-current=page]" })).toBeInTheDocument();
  });
  it("disables Anterior on the first page", () => {
    render(<PaginationBar from={1} to={25} total={200} />);
    expect(screen.getByRole("button", { name: /Anterior/ })).toBeDisabled();
  });
});
