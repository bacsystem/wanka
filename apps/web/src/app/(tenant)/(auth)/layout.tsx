import { ThemeToggle } from "@/components/layout/theme-toggle";

/** Auth pages render outside the app shell (no sidebar / bottom nav). */
export default function AuthLayout({ children }: LayoutProps<"/">) {
  return (
    <div className="relative flex min-h-svh flex-col bg-background">
      <div className="absolute top-3 right-3 z-10"><ThemeToggle /></div>
      {children}
    </div>
  );
}
