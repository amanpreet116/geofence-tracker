import * as RT from "@radix-ui/react-tooltip";
import type { ReactNode } from "react";

export const TooltipProvider = RT.Provider;

/** Small dark label on hover/focus. `disabled` renders just the trigger. */
export function Tooltip({
  content,
  side = "top",
  disabled = false,
  children,
}: {
  content: ReactNode;
  side?: "top" | "right" | "bottom" | "left";
  disabled?: boolean;
  children: ReactNode;
}) {
  if (disabled) return <>{children}</>;
  return (
    <RT.Root>
      <RT.Trigger asChild>{children}</RT.Trigger>
      <RT.Portal>
        <RT.Content
          side={side}
          sideOffset={8}
          className="data-[state=delayed-open]:animate-in z-50 rounded-control border border-strong bg-raised px-2 py-1 text-xs text-fg shadow-float"
        >
          {content}
        </RT.Content>
      </RT.Portal>
    </RT.Root>
  );
}
