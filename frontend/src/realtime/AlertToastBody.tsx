import type { AlertMessage } from "../api/types";
import { CATEGORY_STYLES } from "../lib/categories";
import { formatDateTime } from "../lib/format";

/** Toast description: driver, category, time and coordinates. */
export function AlertToastBody({ alert: a }: { alert: AlertMessage }) {
  return (
    <div className="space-y-0.5 text-xs">
      <div>
        {a.vehicle.driver_name} · {CATEGORY_STYLES[a.geofence.category].label}
      </div>
      <div className="opacity-80">
        {formatDateTime(a.timestamp)} · {a.location.latitude.toFixed(5)},{" "}
        {a.location.longitude.toFixed(5)}
      </div>
    </div>
  );
}
