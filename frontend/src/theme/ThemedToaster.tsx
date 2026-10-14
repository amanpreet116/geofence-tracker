import { Toaster } from "sonner";

import { useTheme } from "./context";

/** Sonner, following the app theme (fuller toast styling comes with the component kit). */
export function ThemedToaster() {
  const { theme } = useTheme();
  return <Toaster position="top-right" theme={theme} richColors closeButton />;
}
