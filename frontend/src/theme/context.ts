import { createContext, useContext } from "react";

import type { ResolvedTheme, ThemePreference } from "./theme";

export interface ThemeValue {
  preference: ThemePreference;
  /** What is actually shown ("system" resolved). */
  theme: ResolvedTheme;
  setPreference: (p: ThemePreference) => void;
}

export const ThemeContext = createContext<ThemeValue | null>(null);

export function useTheme(): ThemeValue {
  const v = useContext(ThemeContext);
  if (!v) throw new Error("useTheme must be used inside <ThemeProvider>");
  return v;
}
