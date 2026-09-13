"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, PlusCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { getBottomNavHrefs, getNavigation, isActivePath, type NavItem } from "@/config/navigation";
import { useOptionalTenant } from "./tenant-context";
import { cn } from "@/lib/utils";

const itemClass =
  "flex flex-1 flex-col items-center justify-center gap-0.5 py-2 text-[11px] font-medium text-muted-foreground transition-colors";
const activeClass = "text-primary";

interface BottomNavProps {
  /** Override the tenant navigation (used by the admin console). */
  items?: NavItem[];
  primaryHrefs?: string[];
  isActive?: (pathname: string, href: string) => boolean;
  fab?: { href: string; label: string } | null;
}

export function BottomNav(props: BottomNavProps) {
  const pathname = usePathname();
  const tenant = useOptionalTenant();
  const industry = tenant?.industry ?? "odontologia";
  const navigation = props.items ?? getNavigation(industry);
  const bottomNavHrefs = props.primaryHrefs ?? getBottomNavHrefs(industry);
  const active = props.isActive ?? isActivePath;
  const primary = navigation.filter((i) => bottomNavHrefs.includes(i.href));
  const more = navigation.filter((i) => !bottomNavHrefs.includes(i.href));
  const fab = props.fab === undefined ? { href: "/ventas/nueva", label: "Nueva venta" } : props.fab;
  const onPos = fab ? pathname === fab.href : true;

  return (
    <>
      {onPos || !fab ? null : (
      <Button
        render={<Link href={fab.href} aria-label={fab.label} />} nativeButton={false}
        size="icon-lg"
        className="fixed right-4 bottom-20 z-30 size-14 rounded-full shadow-lg md:hidden"
      >
        <PlusCircle className="size-6" />
      </Button>
      )}

      <nav
        aria-label="Navegación principal"
        className="fixed inset-x-0 bottom-0 z-30 flex border-t bg-background pb-[env(safe-area-inset-bottom)] md:hidden"
      >
        {primary.map((item) => {
          const isOn = active(pathname, item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={isOn ? "page" : undefined}
              className={cn(itemClass, isOn && activeClass)}
            >
              <item.icon className="size-5" strokeWidth={isOn ? 2 : 1.5} />
              {item.label}
            </Link>
          );
        })}
        <Sheet>
          <SheetTrigger render={<button type="button" className={itemClass} />}>
            <Menu className="size-5" strokeWidth={1.5} />
            Más
          </SheetTrigger>
          <SheetContent side="bottom" className="rounded-t-xl">
            <SheetHeader>
              <SheetTitle>Más opciones</SheetTitle>
            </SheetHeader>
            <ul className="grid grid-cols-2 gap-2 p-4 pt-0">
              {[...more, ...navigation.flatMap((i) => i.children ?? [])].map((item) => (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    className="flex items-center gap-2 rounded-md border px-3 py-2.5 text-sm hover:bg-accent hover:text-accent-foreground"
                  >
                    <item.icon className="size-4" strokeWidth={1.5} />
                    {item.label}
                  </Link>
                </li>
              ))}
            </ul>
          </SheetContent>
        </Sheet>
      </nav>
    </>
  );
}
