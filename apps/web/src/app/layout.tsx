import type { Metadata } from "next";
import { DM_Sans, Figtree, Inter, JetBrains_Mono, Manrope, Plus_Jakarta_Sans } from "next/font/google";
import { ThemeProvider } from "@/components/providers/theme-provider";
import { ScriptDiseno } from "khipu-design-system/script-diseno";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import "./globals.css";

const sans = Inter({ variable: "--font-sans", subsets: ["latin"] });
const jakarta = Plus_Jakarta_Sans({ variable: "--fuente-jakarta", subsets: ["latin"], preload: false });
const manrope = Manrope({ variable: "--fuente-manrope", subsets: ["latin"], preload: false });
const dmSans = DM_Sans({ variable: "--fuente-dmsans", subsets: ["latin"], preload: false });
const figtree = Figtree({ variable: "--fuente-figtree", subsets: ["latin"], preload: false });
const mono = JetBrains_Mono({ variable: "--font-mono", weight: ["400", "500", "600"], subsets: ["latin"] });
const fonts = [sans, jakarta, manrope, dmSans, figtree, mono].map((f) => f.variable).join(" ");

export const metadata: Metadata = {
  title: { default: "Wanka", template: "%s · Wanka" },
  description: "Gestión empresarial multirubro con facturación electrónica SUNAT",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="es-PE"
      suppressHydrationWarning
      className={`${fonts} h-full antialiased`}
    >
      <head>
        <ScriptDiseno />
      </head>
      <body className="flex min-h-full flex-col">
        <ThemeProvider>
          <TooltipProvider>{children}</TooltipProvider>
          <Toaster position="top-right" richColors closeButton />
        </ThemeProvider>
      </body>
    </html>
  );
}
