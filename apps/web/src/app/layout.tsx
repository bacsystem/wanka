import type { Metadata } from "next";
import { DM_Sans, Figtree, Inter, JetBrains_Mono, Manrope, Plus_Jakarta_Sans } from "next/font/google";
import { AppearanceScript } from "@/components/providers/appearance-script";
import { ThemeProvider } from "@/components/providers/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import "./globals.css";

const sans = Inter({ variable: "--font-sans", subsets: ["latin"] });
const jakarta = Plus_Jakarta_Sans({ variable: "--font-opt-jakarta", subsets: ["latin"], preload: false });
const manrope = Manrope({ variable: "--font-opt-manrope", subsets: ["latin"], preload: false });
const dmSans = DM_Sans({ variable: "--font-opt-dmsans", subsets: ["latin"], preload: false });
const figtree = Figtree({ variable: "--font-opt-figtree", subsets: ["latin"], preload: false });
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
        <AppearanceScript />
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