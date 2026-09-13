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
    input.focus();
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
    input.focus();
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
    const input = screen.getByRole("textbox", { name: "Precio" });
    input.focus();
    fireEvent.change(input, { target: { value: "9" } });
    expect(onChange).toHaveBeenLastCalledWith(4);
  });
});

describe("NumberInput resets and integer mode", () => {
  function Resettable() {
    const [value, setValue] = React.useState(3);
    return (
      <>
        <NumberInput value={value} aria-label="Cantidad" onValueChange={setValue} />
        <button type="button" onClick={() => setValue(0)}>Limpiar</button>
      </>
    );
  }
  it("does not fight the user while focused, and shows the parent's value once the field is left", () => {
    render(<Resettable />);
    const input = screen.getByRole("textbox", { name: "Cantidad" }) as HTMLInputElement;
    input.focus();
    fireEvent.change(input, { target: { value: "abc" } }); // commits 0, keeps "abc" while typing
    expect(input.value).toBe("abc");
    fireEvent.click(screen.getByRole("button", { name: "Limpiar" })); // parent sets 0 again while still focused
    expect(input.value).toBe("abc");
    input.blur();
    fireEvent.blur(input);
    expect(input.value).toBe("0");
  });
  it("refreshes stale text of an unfocused field when the parent re-renders", () => {
    render(<Resettable />);
    const input = screen.getByRole("textbox", { name: "Cantidad" }) as HTMLInputElement;
    // Not focused (programmatic change, e.g. a scanner filling the field): text and value drift apart…
    fireEvent.change(input, { target: { value: "abc" } });
    // …and the effect brings the text back to the committed number without waiting for a blur.
    expect(input.value).toBe("0");
  });
  it("truncates decimals and uses the numeric keyboard in integer mode", () => {
    const onChange = vi.fn();
    render(<NumberInput integer value={1} aria-label="Piezas" onValueChange={onChange} />);
    const input = screen.getByRole("textbox", { name: "Piezas" }) as HTMLInputElement;
    expect(input.inputMode).toBe("numeric");
    input.focus();
    fireEvent.change(input, { target: { value: "2.7" } });
    expect(onChange).toHaveBeenLastCalledWith(2);
  });
});
