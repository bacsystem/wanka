import * as React from "react";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

/**
 * Label + control + optional help text. If the child is a single element without an `id`,
 * one is generated and wired to the label so the control is accessible by its label text.
 */
export function Field({
  label,
  htmlFor,
  help,
  children,
  className,
}: {
  label: string;
  htmlFor?: string;
  help?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  const autoId = React.useId();
  let control = children;
  let id = htmlFor;
  if (!id && React.isValidElement(children)) {
    const props = children.props as { id?: string };
    id = props.id ?? autoId;
    if (!props.id) control = React.cloneElement(children as React.ReactElement<{ id?: string }>, { id });
  }
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <Label htmlFor={id} className="text-xs font-medium text-muted-foreground">
        {label}
      </Label>
      {control}
      {help ? <p className="text-xs text-muted-foreground">{help}</p> : null}
    </div>
  );
}
