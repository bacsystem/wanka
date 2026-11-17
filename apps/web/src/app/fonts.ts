import localFont from "next/font/local";

// Self-hosted woff2 files (latin subset, OFL licences next to them, see ./fonts/README.md): the build never downloads fonts.
const sans = localFont({ src: "./fonts/inter-latin-wght-normal.woff2", weight: "100 900", variable: "--font-sans" });
const jakarta = localFont({ src: "./fonts/plus-jakarta-sans-latin-wght-normal.woff2", weight: "200 800", variable: "--fuente-jakarta", preload: false });
const manrope = localFont({ src: "./fonts/manrope-latin-wght-normal.woff2", weight: "200 800", variable: "--fuente-manrope", preload: false });
const dmSans = localFont({ src: "./fonts/dm-sans-latin-wght-normal.woff2", weight: "100 1000", variable: "--fuente-dmsans", preload: false });
const figtree = localFont({ src: "./fonts/figtree-latin-wght-normal.woff2", weight: "300 900", variable: "--fuente-figtree", preload: false });
const mono = localFont({ src: "./fonts/jetbrains-mono-latin-wght-normal.woff2", weight: "100 800", variable: "--font-mono" });
/** CSS classes that define the font variables on <html>. */
export const fonts = [sans, jakarta, manrope, dmSans, figtree, mono].map((f) => f.variable).join(" ");
