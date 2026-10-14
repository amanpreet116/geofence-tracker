import { useEffect, useMemo, useState, type ReactNode } from "react";

import { ThemeContext } from "./context";
import {
  applyTheme,
  readPreference,
  resolve,
  systemTheme,
  THEME_STORAGE_KEY,
  type ResolvedTheme,
  type ThemePreference,
} from "./theme";

/** Theme preference (system / light / dark), persisted in localStorage. */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreference] = useState<ThemePreference>(readPreference);
  const [system, setSystem] = useState<ResolvedTheme>(systemTheme);
  const theme = preference === "system" ? system : resolve(preference);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  useEffect(() => {
    try {
      localStorage.setItem(THEME_STORAGE_KEY, preference);
    } catch {
      /* storage unavailable: still works for this session */
    }
  }, [preference]);

  // Follow OS changes while on "system".
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setSystem(mq.matches ? "dark" : "light");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const value = useMemo(() => ({ preference, theme, setPreference }), [preference, theme]);
  return <ThemeContext value={value}>{children}</ThemeContext>;
}
