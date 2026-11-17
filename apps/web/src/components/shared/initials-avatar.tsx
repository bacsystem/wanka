import { Avatar } from "khipu-design-system/ui/avatar";

const SIZE = { sm: "sm", md: "default", lg: "lg", xl: "xl" } as const;
const TONE = { accent: "accent", primary: "primary", muted: "secondary" } as const;

interface InitialsAvatarProps {
  initials: string;
  size?: "sm" | "md" | "lg" | "xl";
  tone?: "accent" | "primary" | "muted";
  /** @deprecated No effect: the DS avatar is always round (ADR 0010). */
  shape?: "circle" | "square";
  className?: string;
}

/**
 * Initials avatar used in tables, headers and menus.
 * @deprecated Temporary W5a adapter over the DS `Avatar`. Use `Avatar` directly; deleted in W8.
 */
export function InitialsAvatar({ initials, size = "md", tone = "accent", className }: InitialsAvatarProps) {
  return <Avatar nombre={initials} tamano={SIZE[size]} tono={TONE[tone]} className={className} />;
}
