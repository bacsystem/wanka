import { fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";
import { NumberInput } from "./number-input";

function Harness({ onValueChange, max }: { onValueChange: (n: number) => void; max?: number }) {
  const [value, setValue] = React.useState(10);
  return <NumberInput value={value} max={max} aria-label="Precio" onValueChange={(n) => { setValue(n); onValueChange(n); }} />;
}

describe("NumberInput", () => {
  it("keeps the typed text while committing the parsed number", () => {
    const onChange = vi.fn();
    render(<Harness onValueChange={onChange} />);
    const input = screen.getByRole("textbox", { name: "Precio" }) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "12." } });
    expect(input.value).toBe("12.");
    expect(onChange).toHaveBeenLastCalledWith(12);
    fireEvent.change(input, { target: { value: "12.5" } });
    expect(input.value).toBe("12.5");
    expect(onChange).toHaveBeenLastCalledWith(12.5);
  });
  it("accepts a decimal comma, shows empty when cleared and normalizes on blur", () => {
    const onChange = vi.fn();
    render(<Harness onValueChange={onChange} />);
    const input = screen.getByRole("textbox", { name: "Precio" }) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "" } });
    expect(input.value).toBe("");
    expect(onChange).toHaveBeenLastCalledWith(0);
    fireEvent.change(input, { target: { value: "1,5" } });
    expect(onChange).toHaveBeenLastCalledWith(1.5);
    fireEvent.blur(input);
    expect(input.value).toBe("1.5");
  });
  it("clamps to max", () => {
    const onChange = vi.fn();
    render(<Harness onValueChange={onChange} max={4} />);
    fireEvent.change(screen.getByRole("textbox", { name: "Precio" }), { target: { value: "9" } });
    expect(onChange).toHaveBeenLastCalledWith(4);
  });
});
