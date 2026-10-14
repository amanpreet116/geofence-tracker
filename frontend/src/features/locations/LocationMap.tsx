import { useFleetLocations, useGeofences } from "../../api/hooks";
import { FleetMap, type MapFocus } from "../../components/map/FleetMap";
import { PickLocationLayer } from "../../components/map/PickLocationLayer";
import { ErrorState } from "../../components/ui";
import type { LocationDraft } from "./useLocationDraft";

/**
 * Fleet map wired to the location panel: while `picking`, map clicks set the
 * draft position (polygons and markers stop catching clicks). Vehicles that
 * have never reported get a hint instead of a marker.
 */
export function LocationMap({
  draft,
  picking,
  focus,
  autoFit = true,
}: {
  draft: LocationDraft;
  picking: boolean;
  focus: MapFocus | null;
  autoFit?: boolean;
}) {
  const geofences = useGeofences();
  const fleet = useFleetLocations();

  if (geofences.isError || fleet.isError) {
    return (
      <ErrorState
        error={geofences.error ?? fleet.error}
        onRetry={() => {
          void geofences.refetch();
          void fleet.refetch();
        }}
      />
    );
  }

  const unlocated = (fleet.data ?? []).filter((v) => !v.current_location);

  return (
    <div className="relative h-full">
      <FleetMap
        geofences={geofences.data ?? []}
        vehicles={fleet.data}
        focus={focus}
        interactive={!picking}
        autoFit={autoFit}
      >
        <PickLocationLayer active={picking} point={draft.point} onPick={draft.setPoint} />
      </FleetMap>
      {unlocated.length > 0 && (
        // Above Leaflet's panes (z-index 400-700).
        <div className="pointer-events-none absolute right-2 bottom-8 left-2 z-[800] flex justify-center">
          <p className="max-w-full rounded-md bg-white/95 px-3 py-1.5 text-xs text-slate-700 shadow ring-1 ring-slate-200">
            <span className="font-medium">
              {unlocated.length} vehicle{unlocated.length === 1 ? "" : "s"} without a location:
            </span>{" "}
            {unlocated.map((v) => v.vehicle_number).join(", ")} — select one and click the map to
            place it.
          </p>
        </div>
      )}
    </div>
  );
}
