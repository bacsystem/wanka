"use client"

import { Tabs as TabsPrimitive } from "@base-ui/react/tabs"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "cn"

function Tabs({
  className,
  orientation = "horizontal",
  ...props
}: TabsPrimitive.Root.Props) {
  return (
    <TabsPrimitive.Root
      data-slot="tabs"
      data-orientation={orientation}
      className={cn(
        "group/tabs flex min-w-0 max-w-full gap-2 data-horizontal:flex-col",
        className
      )}
      {...props}
    />
  )
}

const tabsListVariants = cva(
  "group/tabs-list no-scrollbar relative isolate inline-flex w-fit max-w-full items-center justify-start overflow-x-auto rounded-full p-[3px] text-muted-foreground group-data-horizontal/tabs:min-h-control group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col data-[variant=line]:rounded-none",
  {
    variants: {
      variant: {
        default: "bg-secondary",
        line: "gap-4 bg-transparent p-0 shadow-[inset_0_-1px_0_var(--color-border)]",
        pills: "h-auto gap-1 bg-secondary p-[3px] group-data-horizontal/tabs:h-auto [&>[data-slot=tabs-trigger]]:h-7 [&>[data-slot=tabs-trigger]]:flex-none [&>[data-slot=tabs-trigger]]:rounded-full [&>[data-slot=tabs-trigger]]:px-3.5 [&>[data-slot=tabs-trigger]]:font-semibold",
        card: "h-auto w-full gap-1 rounded-xl border border-borde-tarjeta bg-card p-1.5 shadow-tarjeta group-data-horizontal/tabs:h-auto [&>[data-slot=tabs-trigger]]:h-control [&>[data-slot=tabs-trigger]]:flex-none [&>[data-slot=tabs-trigger]]:rounded-full [&>[data-slot=tabs-trigger]]:px-3.5 [&>[data-slot=tabs-trigger]]:font-semibold",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function TabsList({
  className,
  variant = "default",
  children,
  ...props
}: TabsPrimitive.List.Props & VariantProps<typeof tabsListVariants>) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      data-variant={variant}
      className={cn(tabsListVariants({ variant }), className)}
      {...props}
    >
      {children}
      <TabsPrimitive.Indicator
        data-slot="tabs-indicator"
        className={cn(
          "pointer-events-none absolute -z-10 left-(--active-tab-left) w-(--active-tab-width) transition-[left,width,top,height] duration-(--duration-medium) ease-salida motion-reduce:transition-none",
          variant === "line"
            ? "bottom-0 h-0.5 rounded-full bg-foreground"
            : variant === "card"
              ? "top-(--active-tab-top) h-(--active-tab-height) rounded-full bg-acento-suave"
              : "top-(--active-tab-top) h-(--active-tab-height) rounded-full bg-card shadow-xs ring-1 ring-border dark:bg-input"
        )}
      />
    </TabsPrimitive.List>
  )
}

function TabsTrigger({ className, ...props }: TabsPrimitive.Tab.Props) {
  return (
    <TabsPrimitive.Tab
      data-slot="tabs-trigger"
      className={cn(
        "relative inline-flex h-7 flex-1 items-center justify-center gap-1.5 rounded-full border border-transparent px-3 py-0.5 text-control font-medium whitespace-nowrap text-muted-foreground transition-colors duration-(--duration-fast) ease-salida outline-none group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2 aria-disabled:pointer-events-none aria-disabled:opacity-50 data-active:font-semibold data-active:text-foreground group-data-[variant=card]/tabs-list:data-active:text-acento-superficie-texto [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    />
  )
}

function TabsContent({ className, ...props }: TabsPrimitive.Panel.Props) {
  return (
    <TabsPrimitive.Panel
      data-slot="tabs-content"
      className={cn("flex-1 text-sm outline-none", className)}
      {...props}
    />
  )
}

export { Tabs, TabsList, TabsTrigger, TabsContent, tabsListVariants }
