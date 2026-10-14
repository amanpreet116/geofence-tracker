import { useEffect } from "react";
import { CircleMarker, Tooltip, useMap, useMapEvents } from "react-leaflet";

import type { LatLng } from "../../api/types";
import { round6 } from "../../lib/coordinates";

/**
 * While `active`, a map click reports its position (rounded to 6 decimals)
 * and the chosen point is shown as a target ring until submitted.
 */
export function PickLocationLayer({
  active,
  point,
  onPick,
}: {
  active: boolean;
  point: LatLng | null;
  onPick: (p: LatLng) => void;
}) {
  const map = useMap();
  useMapEvents({
    click(e) {
      if (active) onPick([round6(e.latlng.lat), round6(e.latlng.lng)]);
    },
  });

  useEffect(() => {
    const el = map.getContainer();
    el.style.cursor = active ? "crosshair" : "";
    return () => {
      el.style.cursor = "";
    };
  }, [map, active]);

  if (!point) return null;
  return (
    <CircleMarker
      center={point}
      radius={9}
      interactive={false}
      pathOptions={{
        color: "#7c3aed",
        weight: 3,
        fillColor: "#7c3aed",
        fillOpacity: 0.25,
        dashArray: "3 3",
      }}
    >
      <Tooltip permanent direction="top" offset={[0, -10]}>
        New position
      </Tooltip>
    </CircleMarker>
  );
}
