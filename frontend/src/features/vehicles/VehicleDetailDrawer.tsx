import { useVehicleLocation, useVehicles } from "../../api/hooks";
import { Button, CategoryBadge, Drawer, ErrorState, LoadingState } from "../../components/ui";
import { formatDateTime } from "../../lib/format";

/** Vehicle detail from GET /vehicles/location/{vehicle_id} (+ registration info). */
export function VehicleDetailDrawer({
  vehicleId,
  onClose,
  onUpdateLocation,
}: {
  vehicleId: string | null;
  onClose: () => void;
  onUpdateLocation?: (vehicleId: string) => void;
}) {
  const location = useVehicleLocation(vehicleId ?? undefined);
  const { data: vehicles } = useVehicles();
  const vehicle = vehicles?.find((v) => v.id === vehicleId);
  const data = location.data;

  return (
    <Drawer
      open={vehicleId !== null}
      onClose={onClose}
      title={data?.vehicle_number ?? vehicle?.vehicle_number ?? "Vehicle"}
      footer={
        vehicleId &&
        onUpdateLocation && (
          <Button onClick={() => onUpdateLocation(vehicleId)}>Update location</Button>
        )
      }
    >
      {location.isPending ? (
        <LoadingState />
      ) : location.isError ? (
        <ErrorState error={location.error} onRetry={() => void location.refetch()} />
      ) : !data ? null : (
        <div className="space-y-6 text-sm">
          {vehicle && (
            <dl className="grid grid-cols-[7rem_1fr] gap-y-2">
              <dt className="text-slate-500">Driver</dt>
              <dd className="text-slate-900">{vehicle.driver_name}</dd>
              <dt className="text-slate-500">Type</dt>
              <dd className="text-slate-900 capitalize">{vehicle.vehicle_type}</dd>
              <dt className="text-slate-500">Phone</dt>
              <dd>
                <a href={`tel:${vehicle.phone}`} className="text-blue-600 hover:text-blue-700">
                  {vehicle.phone}
                </a>
              </dd>
              <dt className="text-slate-500">Registered</dt>
              <dd className="text-slate-900">{formatDateTime(vehicle.created_at)}</dd>
              <dt className="text-slate-500">ID</dt>
              <dd className="font-mono text-xs text-slate-600">{vehicle.id}</dd>
            </dl>
          )}

          <section>
            <h3 className="mb-2 font-semibold text-slate-900">Current location</h3>
            {data.current_location ? (
              <div className="rounded-md bg-slate-50 p-3 ring-1 ring-slate-200">
                <div className="font-mono text-xs text-slate-800">
                  {data.current_location.latitude}, {data.current_location.longitude}
                </div>
                <div className="mt-1 text-xs text-slate-500">
                  Reported {formatDateTime(data.current_location.timestamp)}
                </div>
              </div>
            ) : (
              <p className="text-slate-500">This vehicle hasn't reported a location yet.</p>
            )}
          </section>

          <section>
            <h3 className="mb-2 font-semibold text-slate-900">Current geofences</h3>
            {data.current_geofences.length === 0 ? (
              <p className="text-slate-500">Not inside any geofence.</p>
            ) : (
              <ul className="space-y-2">
                {data.current_geofences.map((g) => (
                  <li
                    key={g.geofence_id}
                    className="flex items-center justify-between gap-2 rounded-md p-2 ring-1 ring-slate-200"
                  >
                    <span className="font-medium text-slate-800">{g.geofence_name}</span>
                    <CategoryBadge category={g.category} />
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      )}
    </Drawer>
  );
}
