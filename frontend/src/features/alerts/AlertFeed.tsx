import type { AlertMessage } from "../../api/types";
import { CategoryBadge, EmptyState, LoadingState } from "../../components/ui";
import { BellIcon, EntryIcon, ExitIcon } from "../../components/ui/icons";
import { cn } from "../../lib/cn";
import { formatDateTime, formatRelative } from "../../lib/format";
import { useNow } from "../../lib/useNow";
import { isCritical } from "../../realtime/alerts";

/**
 * Newest-first alert feed. Relative times refresh on their own; clicking an
 * alert calls `onSelect` (e.g. to fly the map to it).
 */
export function AlertFeed({
  alerts,
  loading = false,
  onSelect,
}: {
  alerts: AlertMessage[];
  loading?: boolean;
  onSelect?: (alert: AlertMessage) => void;
}) {
  const now = useNow();

  if (loading && alerts.length === 0) return <LoadingState label="Loading alerts…" />;
  if (alerts.length === 0) {
    return (
      <EmptyState
        icon={<BellIcon />}
        title="No alerts yet"
        description="Alerts appear here the moment a vehicle enters or leaves a geofence with an alert rule."
      />
    );
  }

  return (
    <ul className="space-y-2" aria-label="Alerts, newest first">
      {alerts.map((a) => {
        const critical = isCritical(a);
        const entry = a.event_type === "entry";
        const Icon = entry ? EntryIcon : ExitIcon;
        const body = (
          <>
            <span
              className={cn(
                "mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full",
                critical
                  ? "bg-red-100 text-red-700"
                  : entry
                    ? "bg-blue-100 text-blue-700"
                    : "bg-amber-100 text-amber-800",
              )}
              aria-hidden="true"
            >
              <Icon className="size-4" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="flex items-baseline justify-between gap-2">
                <span className="truncate font-medium text-slate-900">
                  {a.vehicle.vehicle_number}{" "}
                  <span className="font-normal text-slate-600">
                    {entry ? "entered" : "left"} {a.geofence.geofence_name}
                  </span>
                </span>
                <time
                  dateTime={a.timestamp}
                  title={formatDateTime(a.timestamp)}
                  className="shrink-0 text-xs text-slate-500"
                >
                  {formatRelative(a.timestamp, now)}
                </time>
              </span>
              <span className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-slate-500">
                <CategoryBadge category={a.geofence.category} />
                <span>{a.vehicle.driver_name}</span>
                <span className="font-mono">
                  {a.location.latitude.toFixed(5)}, {a.location.longitude.toFixed(5)}
                </span>
              </span>
            </span>
          </>
        );
        const itemClass = cn(
          "flex w-full gap-3 rounded-lg bg-white p-3 text-left ring-1",
          critical ? "ring-red-200" : "ring-slate-200",
        );
        return (
          <li key={a.event_id}>
            {onSelect ? (
              <button
                type="button"
                onClick={() => onSelect(a)}
                className={cn(itemClass, "transition hover:ring-slate-400")}
                title="Show on map"
              >
                {body}
              </button>
            ) : (
              <div className={itemClass}>{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
