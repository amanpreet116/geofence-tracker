import { useGeofences } from "../api/hooks";
import { CATEGORY_STYLES } from "../lib/categories";
import { cn } from "../lib/cn";

/** Geofence ids rendered as chips colored by category (names from the cached list). */
export function GeofenceChips({
  ids,
  empty = "Not inside any geofence",
}: {
  ids: string[];
  empty?: string;
}) {
  const { data: geofences } = useGeofences();
  if (ids.length === 0) return <span className="text-sm text-slate-500">{empty}</span>;
  return (
    <span className="flex flex-wrap gap-1">
      {ids.map((id) => {
        const g = geofences?.find((x) => x.id === id);
        return (
          <span
            key={id}
            title={g ? CATEGORY_STYLES[g.category].label : undefined}
            className={cn(
              "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset",
              g
                ? CATEGORY_STYLES[g.category].badge
                : "bg-slate-100 text-slate-700 ring-slate-500/20",
            )}
          >
            {g?.name ?? id}
          </span>
        );
      })}
    </span>
  );
}
