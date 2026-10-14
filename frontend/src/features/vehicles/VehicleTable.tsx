import { useFleetLocations, useVehicles } from "../../api/hooks";
import type { FleetVehicle } from "../../api/types";
import { GeofenceChips } from "../../components/GeofenceChips";
import { Badge, Button, EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { TruckIcon } from "../../components/ui/icons";
import { cn } from "../../lib/cn";
import { formatDateTime } from "../../lib/format";

interface Props {
  selectedId: string;
  onUpdateLocation: (vehicleId: string) => void;
  onShowDetails: (vehicleId: string) => void;
  onRegister: () => void;
}

/**
 * Vehicles with their current location and geofences (GET /fleet/locations,
 * joined with GET /vehicles for the phone number). Table on wide screens,
 * cards on narrow ones.
 */
export function VehicleTable({ selectedId, onUpdateLocation, onShowDetails, onRegister }: Props) {
  const fleet = useFleetLocations();
  const { data: vehicles } = useVehicles();
  const phoneOf = (id: string) => vehicles?.find((v) => v.id === id)?.phone;

  if (fleet.isPending) return <LoadingState label="Loading vehicles…" />;
  if (fleet.isError) {
    return (
      <ErrorState
        error={fleet.error}
        title="Couldn't load vehicles"
        onRetry={() => void fleet.refetch()}
      />
    );
  }
  if (fleet.data.length === 0) {
    return (
      <EmptyState
        icon={<TruckIcon />}
        title="No vehicles yet"
        description="Register a vehicle to start tracking it."
        action={<Button onClick={onRegister}>Register vehicle</Button>}
      />
    );
  }

  const actions = (v: FleetVehicle) => (
    <div className="flex gap-1">
      <Button size="sm" variant="secondary" onClick={() => onUpdateLocation(v.vehicle_id)}>
        Update location
      </Button>
      <Button size="sm" variant="ghost" onClick={() => onShowDetails(v.vehicle_id)}>
        Details
      </Button>
    </div>
  );

  const location = (v: FleetVehicle) =>
    v.current_location ? (
      <>
        <div className="font-mono text-xs text-slate-700">
          {v.current_location.latitude.toFixed(5)}, {v.current_location.longitude.toFixed(5)}
        </div>
        <div className="text-xs text-slate-500">{formatDateTime(v.current_location.timestamp)}</div>
      </>
    ) : (
      <Badge>No location yet</Badge>
    );

  return (
    <>
      {/* Wide screens: table */}
      <div className="hidden overflow-x-auto rounded-lg bg-white ring-1 ring-slate-200 md:block">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-50 text-left text-xs font-medium tracking-wide text-slate-500 uppercase">
            <tr>
              <th className="px-4 py-3">Vehicle</th>
              <th className="px-4 py-3">Driver</th>
              <th className="px-4 py-3">Current location</th>
              <th className="px-4 py-3">Current geofences</th>
              <th className="px-4 py-3">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {fleet.data.map((v) => (
              <tr key={v.vehicle_id} className={cn(v.vehicle_id === selectedId && "bg-blue-50/60")}>
                <td className="px-4 py-3">
                  <div className="font-medium whitespace-nowrap text-slate-900">
                    {v.vehicle_number}
                  </div>
                  <div className="text-xs text-slate-500 capitalize">{v.vehicle_type}</div>
                </td>
                <td className="px-4 py-3">
                  <div className="text-slate-900">{v.driver_name}</div>
                  <div className="text-xs text-slate-500">{phoneOf(v.vehicle_id)}</div>
                </td>
                <td className="px-4 py-3">{location(v)}</td>
                <td className="px-4 py-3">
                  <GeofenceChips ids={v.current_geofence_ids} empty="—" />
                </td>
                <td className="px-4 py-3">{actions(v)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Narrow screens: cards */}
      <ul className="space-y-2 md:hidden">
        {fleet.data.map((v) => (
          <li
            key={v.vehicle_id}
            className={cn(
              "space-y-2 rounded-lg bg-white p-3 ring-1",
              v.vehicle_id === selectedId ? "ring-2 ring-blue-500" : "ring-slate-200",
            )}
          >
            <div className="flex items-start justify-between gap-2">
              <div>
                <div className="font-medium text-slate-900">{v.vehicle_number}</div>
                <div className="text-xs text-slate-500">
                  <span className="capitalize">{v.vehicle_type}</span> · {v.driver_name}
                </div>
              </div>
              <div className="text-right">{location(v)}</div>
            </div>
            <GeofenceChips ids={v.current_geofence_ids} />
            {actions(v)}
          </li>
        ))}
      </ul>
    </>
  );
}
