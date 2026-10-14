import * as RP from "@radix-ui/react-popover";
import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

export const Popover = RP.Root;
export const PopoverTrigger = RP.Trigger;
export const PopoverClose = RP.Close;

export function PopoverContent({ className, ...props }: ComponentProps<typeof RP.Content>) {
  return (
    <RP.Portal>
      <RP.Content
        sideOffset={8}
        align="end"
        collisionPadding={8}
        className={cn(
          "z-50 rounded-card border border-subtle bg-raised text-fg shadow-float outline-none",
          className,
        )}
        {...props}
      />
    </RP.Portal>
  );
}
