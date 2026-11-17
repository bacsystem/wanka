import { DEFAULT_FONT, DEFAULT_PALETTE, FONT_KEY, FONTS, PALETTE_KEY, PALETTES } from "@/config/appearance";

const safeJson = (value: unknown) => JSON.stringify(value).replace(/</g, "\\u003c");

const INLINE_SCRIPT = `(function(){var d=document.documentElement.dataset;function r(k,l,f){var x=null;try{x=localStorage.getItem(k)}catch(e){}return l.indexOf(x)>-1?x:f}d.palette=r(${safeJson(PALETTE_KEY)},${safeJson(PALETTES.map((p) => p.value))},${safeJson(DEFAULT_PALETTE)});d.font=r(${safeJson(FONT_KEY)},${safeJson(FONTS.map((f) => f.value))},${safeJson(DEFAULT_FONT)})})()`;

export function AppearanceScript() {
  return <script dangerouslySetInnerHTML={{ __html: INLINE_SCRIPT }} />;
}