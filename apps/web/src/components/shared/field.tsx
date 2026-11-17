import * as React from "react";
import { Campo } from "khipu-design-system/formularios/campo";

/**
 * Label + control + optional help text. If the child is a single element without an `id`,
 * one is generated and wired to the label so the control is accessible by its label text.
 * @deprecated Temporary W5a adapter over `Campo`. Use `Campo` directly (it needs the control's `id`); deleted in W8.
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
  let id = htmlFor ?? autoId;
  if (!htmlFor && React.isValidElement(children)) {
    const props = children.props as { id?: string };
    id = props.id ?? autoId;
    if (!props.id) control = React.cloneElement(children as React.ReactElement<{ id?: string }>, { id });
  }
  return (
    <Campo id={id} etiqueta={label} ayuda={help} className={className}>
      {control}
    </Campo>
  );
}
