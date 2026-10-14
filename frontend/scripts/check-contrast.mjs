// Checks WCAG contrast of every text token against the surfaces it is used
// on, in both themes, by parsing src/styles/tokens.css. Semi-transparent
// backgrounds (badge fills, soft status backgrounds) are composited over the
// surface first. Exits 1 if any pair is below its threshold.
//
//   node scripts/check-contrast.mjs
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("../src/styles/tokens.css", import.meta.url), "utf8");
const block = (selector) => {
  const start = css.indexOf(`${selector} {`);
  const body = css.slice(start, css.indexOf("\n}", start));
  return Object.fromEntries([...body.matchAll(/--([\w-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]));
};
const dark = block(":root");
const themes = { dark, light: { ...dark, ...block(":root.light") } };

const parse = (c) => {
  const hex = c.match(/^#([0-9a-f]{6})$/i);
  if (hex) return [0, 2, 4].map((i) => parseInt(hex[1].slice(i, i + 2), 16)).concat(1);
  const rgba = c.match(/rgba?\(([^)]+)\)/);
  const [r, g, b, a = 1] = rgba[1].split(",").map(Number);
  return [r, g, b, a];
};
const over = (fg, bg) => fg.slice(0, 3).map((c, i) => c * fg[3] + bg[i] * (1 - fg[3])).concat(1);
const lum = ([r, g, b]) =>
  [r, g, b]
    .map((v) => ((v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
    .reduce((s, v, i) => s + v * [0.2126, 0.7152, 0.0722][i], 0);
const ratio = (a, b) => {
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
};

let failures = 0;
for (const [name, t] of Object.entries(themes)) {
  const c = (k) => parse(t[k]);
  const surfaces = ["bg", "surface", "raised", "hover"].map((s) => [s, c(s)]);
  const pairs = [];
  // Body text on every surface (4.5:1).
  for (const fg of ["fg", "fg-muted", "fg-faint", "accent-text"])
    for (const [s, bg] of surfaces) pairs.push([`${fg} on ${s}`, c(fg), bg, 4.5]);
  // Text on solid fills.
  pairs.push(["accent-fg on accent", c("accent-fg"), c("accent"), 4.5]);
  pairs.push(["accent-fg on accent-hover", c("accent-fg"), c("accent-hover"), 4.5]);
  pairs.push(["white on danger-solid", [255, 255, 255, 1], c("danger-solid"), 4.5]);
  // Status text on its soft background (badges, toasts) over surface and raised.
  for (const st of ["success", "warning", "danger", "info"])
    for (const [s, bg] of surfaces.slice(1, 3))
      pairs.push([`${st} on ${st}-soft/${s}`, c(st), over(c(`${st}-soft`), bg), 4.5]);
  // Category badge text on its fill; category stroke vs surface (non-text, 3:1).
  for (const cat of ["restricted", "delivery", "toll", "customer"]) {
    for (const [s, bg] of surfaces.slice(0, 3))
      pairs.push([`cat-${cat}-text on fill/${s}`, c(`cat-${cat}-text`), over(c(`cat-${cat}-fill`), bg), 4.5]);
    pairs.push([`cat-${cat} stroke vs surface (UI 3:1)`, c(`cat-${cat}`), c("surface"), 3]);
  }

  console.log(`\n${name.toUpperCase()}`);
  for (const [label, fg, bg, min] of pairs) {
    const r = ratio(over(fg, bg), bg);
    const ok = r >= min;
    if (!ok) failures++;
    console.log(`  ${ok ? "ok  " : "FAIL"} ${r.toFixed(2).padStart(5)}:1 (min ${min})  ${label}`);
  }
}
console.log(failures ? `\n${failures} pair(s) below WCAG AA` : "\nAll pairs pass WCAG AA");
process.exit(failures ? 1 : 0);
