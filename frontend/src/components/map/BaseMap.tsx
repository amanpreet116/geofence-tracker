import "../../lib/leaflet"; // default marker icon fix; keeps Leaflet in the lazy map chunk
import type { ReactNode } from "react";
import { MapContainer, TileLayer } from "react-leaflet";

import type { LatLng } from "../../api/types";
import { cn } from "../../lib/cn";
import { DEFAULT_CENTER, DEFAULT_ZOOM } from "../../lib/map";

export function BaseMap({
  center = DEFAULT_CENTER,
  zoom = DEFAULT_ZOOM,
  className,
  children,
}: {
  center?: LatLng;
  zoom?: number;
  className?: string;
  children?: ReactNode;
}) {
  return (
    <MapContainer
      center={center}
      zoom={zoom}
      scrollWheelZoom
      className={cn("h-full w-full rounded-lg", className)}
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      {children}
    </MapContainer>
  );
}
