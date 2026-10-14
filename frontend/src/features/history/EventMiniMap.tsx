import { useEffect } from "react";
import { CircleMarker, Polygon, Tooltip, useMap } from "react-leaflet";

import { useGeofences } from "../../api/hooks";
import type { LatLng, Violation } from "../../api/types";
import { EmptyState } from "../../components/ui";
import { GeofenceIcon } from "../../components/ui/icons";
import { BaseMap } from "../../components/map/BaseMap";
import { CATEGORY_STYLES } from "../../lib/categories";
import { formatDateTime } from "../../lib/format";

function Recenter({ point }: { point: LatLng }) {
  const map = useMap();
  useEffect(() => {
    map.setView(point, Math.max(map.getZoom(), 15), { animate: true });
  }, [map, point]);
  return null;
}

/** The selected event's point, with the geofence it crossed. */
export function EventMiniMap({ event }: { event: Violation | null }) {
  const { data: geofences } = useGeofences();
  if (!event) {
    return (
      <EmptyState
        icon={<GeofenceIcon />}
        title="Select an event"
        description="Click a row to see where it happened."
      />
    );
  }
  const point: LatLng = [event.latitude, event.longitude];
  const geofence = geofences?.find((g) => g.id === event.geofence_id);
  const color = geofence ? CATEGORY_STYLES[geofence.category].color : "#475569";

  return (
    <div className="space-y-2">
      <div className="h-64 overflow-hidden rounded-lg ring-1 ring-slate-200 lg:h-80">
        <BaseMap center={point} zoom={15}>
          <Recenter point={point} />
          {geofence && (
            <Polygon
              positions={geofence.coordinates}
              interactive={false}
              pathOptions={{ color, weight: 2, fillOpacity: 0.15 }}
            />
          )}
          <CircleMarker
            center={point}
            radius={8}
            pathOptions={{
              color: event.event_type === "entry" ? "#2563eb" : "#d97706",
              weight: 3,
              fillOpacity: 0.6,
            }}
          >
            <Tooltip permanent direction="top" offset={[0, -8]}>
              {event.vehicle_number} {event.event_type === "entry" ? "entered" : "left"}
            </Tooltip>
          </CircleMarker>
        </BaseMap>
      </div>
      <p className="text-xs text-slate-600">
        <span className="font-medium text-slate-900">{event.vehicle_number}</span>{" "}
        {event.event_type === "entry" ? "entered" : "left"} {event.geofence_name} ·{" "}
        {formatDateTime(event.timestamp)}
      </p>
    </div>
  );
}
