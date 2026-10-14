import * as DM from "@radix-ui/react-dropdown-menu";
import { Check } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "../../lib/cn";

export const DropdownMenu = DM.Root;
export const DropdownMenuTrigger = DM.Trigger;
export const DropdownMenuRadioGroup = DM.RadioGroup;

export function DropdownMenuContent({ className, ...props }: ComponentProps<typeof DM.Content>) {
  return (
    <DM.Portal>
      <DM.Content
        sideOffset={6}
        align="end"
        className={cn(
          "z-50 min-w-44 rounded-card border border-subtle bg-raised p-1 text-sm text-fg shadow-float",
          className,
        )}
        {...props}
      />
    </DM.Portal>
  );
}

const item =
  "relative flex cursor-default items-center gap-2 rounded-control px-2 py-1.5 outline-none select-none " +
  "data-[highlighted]:bg-hover data-[disabled]:opacity-50";

export function DropdownMenuItem({ className, ...props }: ComponentProps<typeof DM.Item>) {
  return <DM.Item className={cn(item, className)} {...props} />;
}

export function DropdownMenuRadioItem({
  className,
  children,
  ...props
}: ComponentProps<typeof DM.RadioItem>) {
  return (
    <DM.RadioItem className={cn(item, "pr-8", className)} {...props}>
      {children}
      <DM.ItemIndicator className="absolute right-2 text-accent-text">
        <Check className="size-4" aria-hidden />
      </DM.ItemIndicator>
    </DM.RadioItem>
  );
}

export function DropdownMenuLabel({ className, ...props }: ComponentProps<typeof DM.Label>) {
  return (
    <DM.Label className={cn("px-2 py-1.5 text-xs font-medium text-muted", className)} {...props} />
  );
}

export function DropdownMenuSeparator() {
  return <DM.Separator className="my-1 h-px bg-subtle" />;
}
