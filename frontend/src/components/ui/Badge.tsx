import type { ReactNode } from "react";

import type { GeofenceCategory, GeofenceEventType } from "../../api/types";
import { CATEGORY_STYLES } from "../../lib/categories";
import { cn } from "../../lib/cn";

const TONES = {
  neutral: "bg-slate-100 text-slate-700 ring-slate-500/20",
  blue: "bg-blue-50 text-blue-700 ring-blue-600/20",
  green: "bg-green-50 text-green-700 ring-green-600/20",
  amber: "bg-amber-50 text-amber-800 ring-amber-600/20",
  red: "bg-red-50 text-red-700 ring-red-600/20",
} as const;

export type BadgeTone = keyof typeof TONES;

export function Badge({
  tone = "neutral",
  className,
  children,
}: {
  tone?: BadgeTone;
  className?: string;
  children: ReactNode;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium whitespace-nowrap ring-1 ring-inset",
        TONES[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}

/** restricted red, delivery blue, toll amber, customer green. */
export function CategoryBadge({ category }: { category: GeofenceCategory }) {
  const style = CATEGORY_STYLES[category];
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium whitespace-nowrap ring-1 ring-inset",
        style.badge,
      )}
    >
      {style.label}
    </span>
  );
}

export function EventTypeBadge({ type }: { type: GeofenceEventType | "both" }) {
  const tone: BadgeTone = type === "entry" ? "blue" : type === "exit" ? "amber" : "neutral";
  return (
    <Badge tone={tone}>
      {type === "both" ? "Entry & exit" : type === "entry" ? "Entry" : "Exit"}
    </Badge>
  );
}
