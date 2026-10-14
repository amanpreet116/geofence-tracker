import { useCallback, useState } from "react";

import type { LatLng } from "../../api/types";
import { toDateTimeLocal } from "../../lib/location";

/**
 * State of the location update panel, shared with the map: clicking the map
 * fills latitude/longitude, typing moves the target marker.
 */
export function useLocationDraft() {
  const [vehicleId, setVehicleId] = useState("");
  const [latitude, setLatitude] = useState("");
  const [longitude, setLongitude] = useState("");
  const [timestamp, setTimestamp] = useState(() => toDateTimeLocal(new Date()));

  const setPoint = useCallback((p: LatLng) => {
    setLatitude(String(p[0]));
    setLongitude(String(p[1]));
    setTimestamp(toDateTimeLocal(new Date())); // a fresh pick means "now"
  }, []);

  const lat = Number(latitude);
  const lng = Number(longitude);
  const point: LatLng | null =
    latitude.trim() !== "" && longitude.trim() !== "" && Math.abs(lat) <= 90 && Math.abs(lng) <= 180
      ? [lat, lng]
      : null;

  return {
    vehicleId,
    setVehicleId,
    latitude,
    setLatitude,
    longitude,
    setLongitude,
    timestamp,
    setTimestamp,
    setPoint,
    point,
    resetPoint: () => {
      setLatitude("");
      setLongitude("");
    },
  };
}

export type LocationDraft = ReturnType<typeof useLocationDraft>;
