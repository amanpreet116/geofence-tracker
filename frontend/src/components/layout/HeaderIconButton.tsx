import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

/** Square icon button for the header: 40px touch target on mobile, 36px on desktop. */
export function HeaderIconButton({ className, ...props }: ComponentProps<"button">) {
  return (
    <button
      type="button"
      className={cn(
        "relative inline-flex size-10 items-center justify-center rounded-control text-muted transition-colors md:size-9",
        "hover:bg-hover hover:text-fg data-[state=open]:bg-hover data-[state=open]:text-fg",
        className,
      )}
      {...props}
    />
  );
}
