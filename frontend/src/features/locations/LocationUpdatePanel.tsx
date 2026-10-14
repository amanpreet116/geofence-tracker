import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { toast } from "sonner";

import { ApiError } from "../../api/client";
import { useFleetLocations, useGeofences, useUpdateLocation } from "../../api/hooks";
import type { UpdateLocationResponse } from "../../api/types";
import { GeofenceChips } from "../../components/GeofenceChips";
import { Button, EmptyState, ErrorState, Input, Select } from "../../components/ui";
import { errorMessage } from "../../lib/errors";
import { formatDateTime } from "../../lib/format";
import type { FieldErrors } from "../../lib/geofenceValidation";
import {
  diffGeofences,
  localToIsoUtc,
  toDateTimeLocal,
  validateLocation,
} from "../../lib/location";
import type { LocationDraft } from "./useLocationDraft";

interface Props {
  draft: LocationDraft;
  /** Called when a vehicle is chosen (e.g. to fly the map to it). */
  onVehicleSelected?: (vehicleId: string) => void;
  onShowDetails?: (vehicleId: string) => void;
}

export function LocationUpdatePanel({ draft, onVehicleSelected, onShowDetails }: Props) {
  const fleet = useFleetLocations();
  const { data: geofences } = useGeofences();
  const update = useUpdateLocation();
  const [errors, setErrors] = useState<FieldErrors>({});
  const [result, setResult] = useState<{
    vehicleNumber: string;
    res: UpdateLocationResponse;
  } | null>(null);

  const vehicles = fleet.data ?? [];
  const selected = vehicles.find((v) => v.vehicle_id === draft.vehicleId);
  const geofenceName = (id: string) => geofences?.find((g) => g.id === id)?.name ?? id;
  const clear = (field: string) =>
    setErrors((e) => Object.fromEntries(Object.entries(e).filter(([k]) => k !== field)));

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const clientErrors = validateLocation(draft);
    setErrors(clientErrors);
    if (Object.keys(clientErrors).length > 0 || !selected) return;

    const before = selected.current_geofence_ids;
    try {
      const res = await update.mutateAsync({
        vehicle_id: draft.vehicleId,
        latitude: Number(draft.latitude),
        longitude: Number(draft.longitude),
        timestamp: localToIsoUtc(draft.timestamp) ?? "",
      });
      setResult({ vehicleNumber: selected.vehicle_number, res });
      draft.resetPoint();

      const { entered, left } = diffGeofences(
        before,
        res.current_geofences.map((g) => g.geofence_id),
      );
      for (const id of left)
        toast.info(`Vehicle ${selected.vehicle_number} left ${geofenceName(id)}`);
      for (const g of res.current_geofences.filter((g) => entered.includes(g.geofence_id))) {
        toast.success(`Vehicle ${selected.vehicle_number} entered ${g.geofence_name}`);
      }
      if (entered.length === 0 && left.length === 0) {
        toast(`Location updated for ${selected.vehicle_number}`);
      }
    } catch (err) {
      if (err instanceof ApiError && Object.keys(err.fieldErrors).length > 0) {
        setErrors(err.fieldErrors);
      }
      toast.error(`Couldn't update location: ${errorMessage(err)}`);
    }
  };

  if (fleet.isError) {
    return (
      <ErrorState
        error={fleet.error}
        title="Couldn't load vehicles"
        onRetry={() => void fleet.refetch()}
      />
    );
  }
  if (fleet.isSuccess && vehicles.length === 0) {
    return (
      <EmptyState
        title="No vehicles yet"
        description="Register a vehicle first, then come back to place it on the map."
        action={
          <Link to="/vehicles" className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Go to Vehicles →
          </Link>
        }
      />
    );
  }

  return (
    <form onSubmit={(e) => void onSubmit(e)} noValidate className="space-y-4">
      <Select
        label="Vehicle"
        required
        value={draft.vehicleId}
        disabled={fleet.isPending}
        error={errors.vehicle_id}
        onChange={(e) => {
          draft.setVehicleId(e.target.value);
          clear("vehicle_id");
          setResult(null);
          if (e.target.value) onVehicleSelected?.(e.target.value);
        }}
      >
        <option value="">{fleet.isPending ? "Loading vehicles…" : "Select a vehicle…"}</option>
        {vehicles.map((v) => (
          <option key={v.vehicle_id} value={v.vehicle_id}>
            {v.vehicle_number} · {v.driver_name}
            {v.current_location ? "" : " (no location yet)"}
          </option>
        ))}
      </Select>

      {selected && (
        <div className="rounded-md bg-slate-50 p-3 text-sm ring-1 ring-slate-200">
          {selected.current_location ? (
            <>
              <div className="text-slate-600">
                Last seen {formatDateTime(selected.current_location.timestamp)} at{" "}
                <span className="font-mono text-xs">
                  {selected.current_location.latitude}, {selected.current_location.longitude}
                </span>
              </div>
              <div className="mt-2">
                <GeofenceChips ids={selected.current_geofence_ids} />
              </div>
            </>
          ) : (
            <p className="text-slate-600">
              No location yet — click the map or enter coordinates to place it for the first time.
            </p>
          )}
          {onShowDetails && (
            <button
              type="button"
              onClick={() => onShowDetails(selected.vehicle_id)}
              className="mt-2 text-xs font-medium text-blue-600 hover:text-blue-700"
            >
              Vehicle details →
            </button>
          )}
        </div>
      )}

      <p className="text-xs text-slate-500">
        {draft.vehicleId
          ? "Click the map to set the position, or type it."
          : "Select a vehicle first."}
      </p>
      <div className="grid grid-cols-2 gap-3">
        <Input
          label="Latitude"
          required
          inputMode="decimal"
          placeholder="37.7849"
          value={draft.latitude}
          error={errors.latitude}
          onChange={(e) => {
            draft.setLatitude(e.target.value);
            clear("latitude");
          }}
        />
        <Input
          label="Longitude"
          required
          inputMode="decimal"
          placeholder="-122.4194"
          value={draft.longitude}
          error={errors.longitude}
          onChange={(e) => {
            draft.setLongitude(e.target.value);
            clear("longitude");
          }}
        />
      </div>
      <div className="flex items-end gap-2">
        <div className="min-w-0 flex-1">
          <Input
            label="Timestamp"
            required
            type="datetime-local"
            step={1}
            value={draft.timestamp}
            error={errors.timestamp}
            hint="Your local time; sent as UTC."
            onChange={(e) => {
              draft.setTimestamp(e.target.value);
              clear("timestamp");
            }}
          />
        </div>
        <Button
          variant="secondary"
          className="mb-5"
          onClick={() => draft.setTimestamp(toDateTimeLocal(new Date()))}
        >
          Now
        </Button>
      </div>

      <Button
        type="submit"
        className="w-full"
        loading={update.isPending}
        disabled={!draft.vehicleId}
      >
        Update location
      </Button>

      {result && (
        <div
          aria-live="polite"
          className="rounded-md bg-green-50 p-3 text-sm ring-1 ring-green-200"
        >
          <div className="font-medium text-green-900">{result.vehicleNumber} is now inside:</div>
          <div className="mt-2">
            <GeofenceChips ids={result.res.current_geofences.map((g) => g.geofence_id)} />
          </div>
        </div>
      )}
    </form>
  );
}
