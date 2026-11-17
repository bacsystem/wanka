import { DM_Sans, Figtree, Inter, JetBrains_Mono, Manrope, Plus_Jakarta_Sans } from "next/font/google";

const sans = Inter({ variable: "--font-sans", subsets: ["latin"] });
const jakarta = Plus_Jakarta_Sans({ variable: "--fuente-jakarta", subsets: ["latin"], preload: false });
const manrope = Manrope({ variable: "--fuente-manrope", subsets: ["latin"], preload: false });
const dmSans = DM_Sans({ variable: "--fuente-dmsans", subsets: ["latin"], preload: false });
const figtree = Figtree({ variable: "--fuente-figtree", subsets: ["latin"], preload: false });
const mono = JetBrains_Mono({ variable: "--font-mono", weight: ["400", "500", "600"], subsets: ["latin"] });
/** CSS classes that define the font variables on <html>. */
export const fonts = [sans, jakarta, manrope, dmSans, figtree, mono].map((f) => f.variable).join(" ");
