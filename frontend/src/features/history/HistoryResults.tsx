import { useGeofences } from "../../api/hooks";
import type { Violation } from "../../api/types";
import { CategoryBadge, EventTypeBadge } from "../../components/ui";
import { cn } from "../../lib/cn";
import { formatDateTime } from "../../lib/format";

const coords = (v: Violation) => `${v.latitude.toFixed(5)}, ${v.longitude.toFixed(5)}`;

/** Placeholder rows while the first page loads. */
export function HistorySkeleton({ rows = 8 }: { rows?: number }) {
  return (
    <div role="status" aria-label="Loading events" className="space-y-2">
      {Array.from({ length: rows }, (_, i) => (
        <div
          key={i}
          className="flex animate-pulse gap-4 rounded-lg bg-white p-4 ring-1 ring-slate-200"
        >
          <div className="h-4 w-36 rounded bg-slate-200" />
          <div className="h-4 w-24 rounded bg-slate-200" />
          <div className="h-4 flex-1 rounded bg-slate-200" />
          <div className="hidden h-4 w-16 rounded bg-slate-200 sm:block" />
          <div className="hidden h-4 w-40 rounded bg-slate-200 md:block" />
        </div>
      ))}
    </div>
  );
}

/** Events as a table (≥ md) or cards (mobile). Clicking selects a row for the mini map. */
export function HistoryResults({
  rows,
  selectedId,
  onSelect,
  dimmed,
}: {
  rows: Violation[];
  selectedId: string | null;
  onSelect: (v: Violation) => void;
  /** True while the next page loads (previous page stays visible). */
  dimmed: boolean;
}) {
  const { data: geofences } = useGeofences();
  const categoryOf = (id: string) => geofences?.find((g) => g.id === id)?.category;

  return (
    <div className={cn("transition-opacity", dimmed && "opacity-60")} aria-busy={dimmed}>
      <div className="hidden overflow-x-auto rounded-lg bg-white ring-1 ring-slate-200 md:block">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-50 text-left text-xs font-medium tracking-wide text-slate-500 uppercase">
            <tr>
              <th className="px-4 py-3">Time</th>
              <th className="px-4 py-3">Vehicle</th>
              <th className="px-4 py-3">Geofence</th>
              <th className="px-4 py-3">Event</th>
              <th className="px-4 py-3">Coordinates</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {rows.map((v) => {
              const category = categoryOf(v.geofence_id);
              const selected = v.id === selectedId;
              return (
                <tr
                  key={v.id}
                  onClick={() => onSelect(v)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onSelect(v);
                    }
                  }}
                  tabIndex={0}
                  aria-selected={selected}
                  className={cn(
                    "cursor-pointer outline-none hover:bg-slate-50 focus-visible:bg-blue-50",
                    selected && "bg-blue-50 hover:bg-blue-50",
                  )}
                >
                  <td className="px-4 py-3 whitespace-nowrap text-slate-700">
                    <time dateTime={v.timestamp}>{formatDateTime(v.timestamp)}</time>
                  </td>
                  <td className="px-4 py-3 font-medium whitespace-nowrap text-slate-900">
                    {v.vehicle_number}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-slate-900">{v.geofence_name}</span>
                      {category && <CategoryBadge category={category} />}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <EventTypeBadge type={v.event_type} />
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-slate-600">{coords(v)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <ul className="space-y-2 md:hidden">
        {rows.map((v) => {
          const category = categoryOf(v.geofence_id);
          return (
            <li key={v.id}>
              <button
                type="button"
                onClick={() => onSelect(v)}
                aria-pressed={v.id === selectedId}
                className={cn(
                  "w-full space-y-1.5 rounded-lg bg-white p-3 text-left ring-1",
                  v.id === selectedId ? "ring-2 ring-blue-500" : "ring-slate-200",
                )}
              >
                <span className="flex items-center justify-between gap-2">
                  <span className="font-medium text-slate-900">{v.vehicle_number}</span>
                  <EventTypeBadge type={v.event_type} />
                </span>
                <span className="flex flex-wrap items-center gap-2 text-sm text-slate-700">
                  {v.geofence_name}
                  {category && <CategoryBadge category={category} />}
                </span>
                <span className="flex flex-wrap justify-between gap-2 text-xs text-slate-500">
                  <time dateTime={v.timestamp}>{formatDateTime(v.timestamp)}</time>
                  <span className="font-mono">{coords(v)}</span>
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
