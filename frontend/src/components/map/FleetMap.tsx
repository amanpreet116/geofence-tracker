import L from "leaflet";
import { useEffect, useMemo, useRef, type ReactNode } from "react";
import { Marker, Polygon, Popup, useMap } from "react-leaflet";

import { useGeofences } from "../../api/hooks";
import type { FleetVehicle, Geofence, LatLng } from "../../api/types";
import { CATEGORY_STYLES } from "../../lib/categories";
import { formatDateTime } from "../../lib/format";
import { CategoryBadge } from "../ui";
import { AlertPulseLayer } from "./AlertPulseLayer";
import { BaseMap } from "./BaseMap";

/** Request to fly to a geofence or a point; bump `nonce` to fly there again. */
export type MapFocus = { nonce: number } & ({ geofenceId: string } | { point: LatLng });

export interface FleetMapProps {
  geofences: Geofence[];
  vehicles?: FleetVehicle[];
  /** Drawn emphasized (hover/selection). */
  highlightedId?: string | null;
  focus?: MapFocus | null;
  onGeofenceClick?: (id: string) => void;
  /** false while drawing: polygons ignore clicks so they reach the map. */
  interactive?: boolean;
  /** Fit the view to all data on first load (off when opening on a focus). */
  autoFit?: boolean;
  className?: string;
  /** Extra layers, e.g. the drawing layer. */
  children?: ReactNode;
}

/**
 * Geofence polygons (colored by category) and vehicle markers. Fits the
 * view to all data on first load, then leaves the viewport to the user.
 */
export function FleetMap({
  geofences,
  vehicles = [],
  highlightedId,
  focus,
  onGeofenceClick,
  interactive = true,
  autoFit = true,
  className,
  children,
}: FleetMapProps) {
  const allPoints = useMemo<LatLng[]>(
    () => [
      ...geofences.flatMap((g) => g.coordinates),
      ...vehicles.flatMap((v) =>
        v.current_location
          ? [[v.current_location.latitude, v.current_location.longitude] as LatLng]
          : [],
      ),
    ],
    [geofences, vehicles],
  );

  return (
    <BaseMap className={className}>
      {geofences.map((g) => (
        <GeofencePolygon
          // Leaflet reads `interactive` only at creation; remount when it flips.
          key={`${g.id}:${interactive}`}
          geofence={g}
          highlighted={g.id === highlightedId}
          interactive={interactive}
          onClick={onGeofenceClick}
        />
      ))}
      <VehicleMarkers vehicles={vehicles} interactive={interactive} />
      <AlertPulseLayer />
      {children}
      {autoFit && <FitToDataOnce points={allPoints} />}
      <FlyToFocus focus={focus} geofences={geofences} />
    </BaseMap>
  );
}

function GeofencePolygon({
  geofence: g,
  highlighted,
  interactive,
  onClick,
}: {
  geofence: Geofence;
  highlighted: boolean;
  interactive: boolean;
  onClick?: (id: string) => void;
}) {
  const ref = useRef<L.Polygon>(null);
  const color = CATEGORY_STYLES[g.category].color;
  const baseFill = g.category === "restricted_zone" ? 0.25 : 0.15;

  useEffect(() => {
    if (highlighted) ref.current?.bringToFront();
  }, [highlighted]);

  return (
    <Polygon
      ref={ref}
      positions={g.coordinates}
      interactive={interactive}
      pathOptions={{
        color,
        weight: highlighted ? 4 : 2,
        fillOpacity: highlighted ? baseFill + 0.2 : baseFill,
        dashArray: interactive ? undefined : "4 4",
      }}
      eventHandlers={{ click: () => onClick?.(g.id) }}
    >
      {interactive && (
        <Popup>
          <div className="min-w-44 space-y-1.5">
            <div className="font-semibold text-slate-900">{g.name}</div>
            <CategoryBadge category={g.category} />
            {g.description && <p className="!my-0 text-slate-600">{g.description}</p>}
          </div>
        </Popup>
      )}
    </Polygon>
  );
}

function VehicleMarkers({
  vehicles,
  interactive,
}: {
  vehicles: FleetVehicle[];
  interactive: boolean;
}) {
  // Names for current_geofence_ids come from the unfiltered list (cached query).
  const { data: all } = useGeofences();
  const names = useMemo(() => new Map(all?.map((g) => [g.id, g.name])), [all]);

  return (
    <>
      {vehicles.map((v) =>
        v.current_location ? (
          <Marker
            key={`${v.vehicle_id}:${interactive}`}
            position={[v.current_location.latitude, v.current_location.longitude]}
            interactive={interactive}
          >
            <Popup>
              <div className="min-w-44 space-y-1">
                <div className="font-semibold text-slate-900">{v.vehicle_number}</div>
                <div className="text-slate-600">
                  {v.driver_name} · {v.vehicle_type}
                </div>
                <div className="text-xs text-slate-500">
                  Seen {formatDateTime(v.current_location.timestamp)}
                </div>
                <div className="pt-1 text-xs">
                  <span className="font-medium text-slate-700">Inside: </span>
                  {v.current_geofence_ids.length === 0
                    ? "no geofence"
                    : v.current_geofence_ids.map((id) => names.get(id) ?? id).join(", ")}
                </div>
              </div>
            </Popup>
          </Marker>
        ) : null,
      )}
    </>
  );
}

function FitToDataOnce({ points }: { points: LatLng[] }) {
  const map = useMap();
  const done = useRef(false);
  useEffect(() => {
    if (done.current || points.length === 0) return;
    done.current = true;
    map.fitBounds(L.latLngBounds(points), { padding: [32, 32], maxZoom: 15 });
  }, [map, points]);
  return null;
}

function FlyToFocus({ focus, geofences }: { focus?: MapFocus | null; geofences: Geofence[] }) {
  const map = useMap();
  const flown = useRef<number | null>(null);
  useEffect(() => {
    if (!focus || flown.current === focus.nonce) return;
    if ("point" in focus) {
      flown.current = focus.nonce;
      map.flyTo(focus.point, Math.max(map.getZoom(), 15), { duration: 0.8 });
      return;
    }
    const g = geofences.find((x) => x.id === focus.geofenceId);
    if (!g) return; // e.g. just created and the list is still refetching
    flown.current = focus.nonce;
    map.flyToBounds(L.latLngBounds(g.coordinates), {
      padding: [48, 48],
      maxZoom: 16,
      duration: 0.8,
    });
  }, [map, focus, geofences]);
  return null;
}
