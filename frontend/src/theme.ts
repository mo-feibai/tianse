// Maps a status snapshot onto the CSS custom properties the components read.
//
// Colour maths stays in Go (DESIGN 2.7): the frontend receives the already
// compensated accent, ink and xiang for the current temperature and simply hands
// them to the cascade. Doing the arithmetic here would mean two implementations
// of the same curve, and the screen would drift away from the axis.

import type { Status } from "./api";

export interface ThemeVars {
  [key: string]: string;
}

const dark = {
  bg: "#14171C",
  sunk: "#161B20",
  surface: "#1D2025",
  raised: "#202429",
  pressed: "#181B20",
  text: "#E8EEF0",
  shadow: "#0C0E12",
  sheen:
    "linear-gradient(160deg, rgba(232,238,240,0.03) 0%, rgba(232,238,240,0) 40%, rgba(12,14,18,0.07) 100%)",
  edge:
    "inset 0 1px 0 rgba(232,238,240,0.06), inset 0 -1px 0 rgba(12,14,18,0.35)",
  // Hour marks on the sky band. They sit on the gradient, not on the card, so
  // they cannot be one fixed colour: a pale rule vanishes on the light theme's
  // darker band and a dark one vanishes on the dark theme's.
  tick: "rgba(232,238,240,0.10)",
};

const light = {
  bg: "#EDF0F0",
  sunk: "#F0F4F4",
  surface: "#F5F8F8",
  raised: "#FBFCFC",
  pressed: "#F7F9F9",
  text: "#14171C",
  shadow: "#B9C4C6",
  sheen:
    "linear-gradient(160deg, rgba(255,255,255,0.55) 0%, rgba(255,255,255,0) 40%, rgba(20,23,28,0.035) 100%)",
  edge:
    "inset 0 1px 0 rgba(255,255,255,0.9), inset 0 -1px 0 rgba(20,23,28,0.09)",
  tick: "rgba(20,23,28,0.16)",
};

export function themeVars(status: Status): ThemeVars {
  const p = status.SurfaceDark ? dark : light;
  return {
    "--bg": p.bg,
    "--sunk": p.sunk,
    "--surface": p.surface,
    "--raised": p.raised,
    "--pressed": p.pressed,
    "--text": p.text,
    "--shadow": p.shadow,
    "--sheen": p.sheen,
    "--edge": p.edge,
    "--tick": p.tick,
    "--accent": status.Accent,
    "--ink": status.Ink,
    "--xiang": status.Xiang,
  };
}
