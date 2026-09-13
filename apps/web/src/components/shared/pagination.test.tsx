import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PaginationBar, pageWindow } from "./pagination";

describe("pageWindow", () => {
  it("lists every page when there are five or fewer", () => {
    expect(pageWindow(2, 5)).toEqual([1, 2, 3, 4, 5]);
  });
  it("keeps the current page visible in the middle of a long range", () => {
    expect(pageWindow(4, 8)).toEqual([1, "…", 3, 4, 5, "…", 8]);
    expect(pageWindow(1, 8)).toEqual([1, 2, "…", 8]);
    expect(pageWindow(8, 8)).toEqual([1, "…", 7, 8]);
  });
});

describe("PaginationBar", () => {
  it("marks the current page after navigating past the first window", () => {
    render(<PaginationBar from={1} to={25} total={200} />);
    const next = screen.getByRole("button", { name: "Siguiente" });
    fireEvent.click(next); fireEvent.click(next); fireEvent.click(next);
    expect(screen.getByRole("button", { name: "4" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "Última página" })).toBeEnabled();
  });
  it("reports page changes when controlled", () => {
    const onPageChange = vi.fn();
    render(<PaginationBar from={26} to={50} total={200} page={2} onPageChange={onPageChange} />);
    fireEvent.click(screen.getByRole("button", { name: "Última página" }));
    expect(onPageChange).toHaveBeenCalledWith(8);
    expect(screen.getByRole("button", { name: "2" })).toHaveAttribute("aria-current", "page");
  });
});
