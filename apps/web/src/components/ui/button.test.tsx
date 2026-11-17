import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "./button";

describe("Button class merging", () => {
  it.each(["default", "sm", "xs"] as const)(
    "keeps the text color of the default variant at size %s",
    (size) => {
      render(<Button size={size}>Agregar</Button>);
      const classes = screen.getByRole("button", { name: "Agregar" }).className.split(/\s+/);
      expect(classes).toContain("text-primary-foreground");
      expect(classes).toContain("bg-primary");
    },
  );

  it("keeps both the DS typography class and the text color", () => {
    render(<Button size="sm">Agregar</Button>);
    const classes = screen.getByRole("button", { name: "Agregar" }).className.split(/\s+/);
    expect(classes).toContain("text-etiqueta");
    expect(classes).toContain("text-primary-foreground");
  });
});
