import type { IconWeight } from "@phosphor-icons/react";
import type { ComponentType } from "react";

export type IconComponent = ComponentType<{
  className?: string;
  weight?: IconWeight;
  "aria-hidden"?: boolean | "true" | "false";
}>;

export const ICON_WEIGHT: IconWeight = "duotone";
export const ICON_WEIGHT_ACTIVE: IconWeight = "fill";
export const ICON_WEIGHT_FUNCTIONAL: IconWeight = "bold";