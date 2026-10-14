import { cn } from "../../lib/cn";
import type { SocketStatus } from "../../realtime/alertSocket";

const STATUS: Record<
  SocketStatus,
  { label: string; dot: string; text: string; ring: string; pulse?: boolean; title: string }
> = {
  open: {
    label: "Live",
    dot: "bg-accent",
    text: "text-accent-text",
    ring: "border-accent/40 bg-accent-soft",
    pulse: true,
    title: "Receiving real-time alerts",
  },
  connecting: {
    label: "Connecting…",
    dot: "bg-warning",
    text: "text-warning",
    ring: "border-warning-border bg-warning-soft",
    pulse: true,
    title: "Connecting to the alert stream",
  },
  reconnecting: {
    label: "Reconnecting…",
    dot: "bg-warning",
    text: "text-warning",
    ring: "border-warning-border bg-warning-soft",
    pulse: true,
    title: "Connection lost; retrying",
  },
  closed: {
    label: "Offline",
    dot: "bg-danger",
    text: "text-danger",
    ring: "border-danger-border bg-danger-soft",
    title: "Can't reach the alert stream; retrying in the background",
  },
};

/** Live-connection pill: teal "Live" with a pulsing dot, amber reconnecting, red offline. */
export function ConnectionIndicator({ status }: { status: SocketStatus }) {
  const s = STATUS[status];
  return (
    <span
      role="status"
      aria-live="polite"
      title={s.title}
      className={cn(
        "inline-flex h-7 items-center gap-2 rounded-full border px-2.5 text-xs font-medium",
        s.ring,
        s.text,
      )}
    >
      <span className="relative flex size-2" aria-hidden="true">
        {s.pulse && (
          <span
            className={cn(
              "absolute inline-flex size-full animate-ping rounded-full opacity-60",
              s.dot,
            )}
          />
        )}
        <span className={cn("relative inline-flex size-2 rounded-full", s.dot)} />
      </span>
      {s.label}
    </span>
  );
}
