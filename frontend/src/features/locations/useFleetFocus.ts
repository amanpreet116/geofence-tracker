import { useCallback, useState } from "react";

import { useFleetLocations } from "../../api/hooks";
import type { LatLng } from "../../api/types";
import type { MapFocus } from "../../components/map/FleetMap";

/** Map focus state plus helpers to fly to a point or a vehicle's last position. */
export function useFleetFocus(initialPoint?: LatLng | null) {
  const { data: fleet } = useFleetLocations();
  const [focus, setFocus] = useState<MapFocus | null>(() =>
    initialPoint ? { point: initialPoint, nonce: 1 } : null,
  );

  const flyToPoint = useCallback(
    (point: LatLng) => setFocus((f) => ({ point, nonce: (f?.nonce ?? 0) + 1 })),
    [],
  );

  const flyToVehicle = useCallback(
    (vehicleId: string) => {
      const loc = fleet?.find((v) => v.vehicle_id === vehicleId)?.current_location;
      if (!loc) return;
      setFocus((f) => ({ point: [loc.latitude, loc.longitude], nonce: (f?.nonce ?? 0) + 1 }));
    },
    [fleet],
  );

  return { focus, flyToVehicle, flyToPoint };
}
