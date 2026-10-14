import L from "leaflet";
import { useEffect, useRef } from "react";
import { CircleMarker, Polygon, Polyline, useMap, useMapEvents } from "react-leaflet";

import type { LatLng } from "../../api/types";
import { isClosedRing, round6 } from "../../lib/coordinates";

/**
 * Click-to-draw polygon layer built on useMapEvents (no draw plugin).
 * While `active`, each map click adds a vertex; clicking the first vertex
 * (with ≥ 3 points) finishes the ring. Renders a live preview.
 */
export function DrawLayer({
  points,
  color,
  active,
  onAddPoint,
  onFinish,
  fitNonce = 0,
}: {
  points: LatLng[];
  /** Change to fit the map to the current points (e.g. after a text preview). */
  fitNonce?: number;
  color: string;
  active: boolean;
  onAddPoint: (point: LatLng) => void;
  onFinish: () => void;
}) {
  const map = useMap();
  const closed = isClosedRing(points);

  useMapEvents({
    click(e) {
      if (active && !closed) onAddPoint([round6(e.latlng.lat), round6(e.latlng.lng)]);
    },
  });

  const fitted = useRef(fitNonce);
  useEffect(() => {
    if (fitNonce === fitted.current || points.length === 0) return;
    fitted.current = fitNonce;
    map.fitBounds(L.latLngBounds(points), { padding: [48, 48], maxZoom: 16 });
  }, [map, fitNonce, points]);

  useEffect(() => {
    const el = map.getContainer();
    el.style.cursor = active && !closed ? "crosshair" : "";
    return () => {
      el.style.cursor = "";
    };
  }, [map, active, closed]);

  if (points.length === 0) return null;
  const vertices = closed ? points.slice(0, -1) : points;

  return (
    <>
      {closed ? (
        <Polygon
          positions={points}
          interactive={false}
          pathOptions={{ color, weight: 3, fillOpacity: 0.3 }}
        />
      ) : (
        <>
          {points.length >= 3 && (
            // Faint preview of the shape the ring will close into.
            <Polygon
              positions={points}
              interactive={false}
              pathOptions={{ color, weight: 0, fillOpacity: 0.12 }}
            />
          )}
          <Polyline
            positions={points}
            interactive={false}
            pathOptions={{ color, weight: 3, dashArray: "6 6" }}
          />
        </>
      )}
      {vertices.map((p, i) => {
        const isFirst = i === 0;
        const canClose = isFirst && !closed && active && points.length >= 3;
        return (
          <CircleMarker
            // Index keys are right here: vertices are append/pop only.
            key={`${i}:${canClose}`}
            center={p}
            radius={isFirst ? 7 : 5}
            interactive={canClose}
            bubblingMouseEvents={false}
            pathOptions={{
              color,
              weight: 2,
              fillColor: isFirst ? color : "#fff",
              fillOpacity: 1,
            }}
            eventHandlers={canClose ? { click: onFinish } : undefined}
          />
        );
      })}
    </>
  );
}
