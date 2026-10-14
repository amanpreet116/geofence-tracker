import { Monitor, Moon, Sun } from "lucide-react";

import { useTheme } from "../../theme/context";
import type { ThemePreference } from "../../theme/theme";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "../ui/DropdownMenu";
import { Tooltip } from "../ui/Tooltip";
import { HeaderIconButton } from "./HeaderIconButton";

const OPTIONS: { value: ThemePreference; label: string; icon: typeof Sun }[] = [
  { value: "system", label: "System", icon: Monitor },
  { value: "light", label: "Light", icon: Sun },
  { value: "dark", label: "Dark", icon: Moon },
];

/** System / light / dark, persisted (see ThemeProvider). */
export function ThemeToggle() {
  const { preference, theme, setPreference } = useTheme();
  const Icon = preference === "system" ? Monitor : theme === "dark" ? Moon : Sun;
  return (
    <DropdownMenu>
      <Tooltip content="Theme" side="bottom">
        <DropdownMenuTrigger asChild>
          <HeaderIconButton aria-label={`Theme: ${preference}`}>
            <Icon className="size-4" aria-hidden />
          </HeaderIconButton>
        </DropdownMenuTrigger>
      </Tooltip>
      <DropdownMenuContent>
        <DropdownMenuLabel>Theme</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={preference}
          onValueChange={(v) => setPreference(v as ThemePreference)}
        >
          {OPTIONS.map(({ value, label, icon: OptionIcon }) => (
            <DropdownMenuRadioItem key={value} value={value}>
              <OptionIcon className="size-4 text-muted" aria-hidden />
              {label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
