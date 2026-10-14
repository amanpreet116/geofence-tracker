import { Bell, LogIn, LogOut } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router";

import type { AlertMessage } from "../../api/types";
import { cn } from "../../lib/cn";
import { formatDateTime, formatRelative } from "../../lib/format";
import { useNow } from "../../lib/useNow";
import { isCritical } from "../../realtime/alerts";
import { useAlertSocket } from "../../realtime/context";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "../ui/Popover";
import { Tooltip } from "../ui/Tooltip";
import { HeaderIconButton } from "./HeaderIconButton";

const SEEN_KEY = "gt-alerts-seen";
const SHOWN = 8;

function readSeen(): string | null {
  try {
    return localStorage.getItem(SEEN_KEY);
  } catch {
    return null;
  }
}

/**
 * Unread = alerts newer than the last one seen in this dropdown (UI-only
 * state in localStorage). On the first visit, existing history counts as read.
 */
function useUnread(alerts: AlertMessage[], loadingHistory: boolean) {
  const [seen, setSeen] = useState<string | null>(readSeen);
  if (seen === null && !loadingHistory) setSeen(alerts[0]?.event_id ?? "");

  useEffect(() => {
    if (seen === null) return;
    try {
      localStorage.setItem(SEEN_KEY, seen);
    } catch {
      /* ignore */
    }
  }, [seen]);

  const index = seen ? alerts.findIndex((a) => a.event_id === seen) : -1;
  const unread = seen === null ? 0 : index === -1 ? alerts.length : index;
  return { unread, markRead: () => setSeen(alerts[0]?.event_id ?? "") };
}

const categoryText = {
  restricted_zone: "text-cat-restricted-text",
  delivery_zone: "text-cat-delivery-text",
  toll_zone: "text-cat-toll-text",
  customer_area: "text-cat-customer-text",
} as const;

export function AlertsBell() {
  const { alerts, loadingHistory } = useAlertSocket();
  const { unread, markRead } = useUnread(alerts, loadingHistory);
  const navigate = useNavigate();
  const now = useNow(30_000);
  const recent = alerts.slice(0, SHOWN);

  return (
    <Popover onOpenChange={(open) => !open && markRead()}>
      <Tooltip content="Recent alerts" side="bottom">
        <PopoverTrigger asChild>
          <HeaderIconButton aria-label={unread > 0 ? `Alerts, ${unread} unread` : "Alerts"}>
            <Bell className="size-4" aria-hidden />
            {unread > 0 && (
              <span className="absolute top-1 right-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-danger-solid px-1 text-[10px] leading-none font-semibold text-white num">
                {unread > 99 ? "99+" : unread}
              </span>
            )}
          </HeaderIconButton>
        </PopoverTrigger>
      </Tooltip>
      <PopoverContent className="w-[min(22rem,calc(100vw-1rem))] p-0">
        <div className="flex items-center justify-between border-b border-subtle px-3 py-2">
          <span className="text-sm font-medium">Recent alerts</span>
          {unread > 0 && <span className="text-xs text-muted num">{unread} new</span>}
        </div>
        {recent.length === 0 ? (
          <p className="px-3 py-6 text-center text-sm text-muted">
            {loadingHistory ? "Loading…" : "No alerts yet."}
          </p>
        ) : (
          <ul className="max-h-80 overflow-y-auto py-1">
            {recent.map((a, i) => {
              const Icon = a.event_type === "entry" ? LogIn : LogOut;
              return (
                <li key={a.event_id}>
                  <PopoverClose asChild>
                    <button
                      type="button"
                      onClick={() =>
                        navigate("/", {
                          state: { focusPoint: [a.location.latitude, a.location.longitude] },
                        })
                      }
                      className="flex w-full items-start gap-3 px-3 py-2 text-left hover:bg-hover"
                    >
                      <Icon
                        className={cn(
                          "mt-0.5 size-4 shrink-0",
                          isCritical(a) ? "text-danger" : categoryText[a.geofence.category],
                        )}
                        aria-hidden
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm">
                          <span className="font-mono font-medium">{a.vehicle.vehicle_number}</span>{" "}
                          <span className="text-muted">
                            {a.event_type === "entry" ? "entered" : "left"}
                          </span>{" "}
                          {a.geofence.geofence_name}
                        </span>
                        <time
                          dateTime={a.timestamp}
                          title={formatDateTime(a.timestamp)}
                          className="font-mono text-xs text-faint num"
                        >
                          {formatRelative(a.timestamp, now)}
                        </time>
                      </span>
                      {i < unread && (
                        <span
                          className="mt-1.5 size-2 shrink-0 rounded-full bg-accent"
                          aria-label="unread"
                        />
                      )}
                    </button>
                  </PopoverClose>
                </li>
              );
            })}
          </ul>
        )}
        <div className="border-t border-subtle px-3 py-2">
          <PopoverClose asChild>
            <Link to="/alerts" className="text-sm font-medium text-accent-text hover:underline">
              Open alerts →
            </Link>
          </PopoverClose>
        </div>
      </PopoverContent>
    </Popover>
  );
}
