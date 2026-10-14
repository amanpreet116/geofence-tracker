import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { toast } from "sonner";

import { ApiError } from "../../api/client";
import { useCreateAlertRule, useGeofences, useVehicles } from "../../api/hooks";
import type { AlertEventType } from "../../api/types";
import { Button, EmptyState, Select } from "../../components/ui";
import { CATEGORY_STYLES } from "../../lib/categories";
import { errorMessage } from "../../lib/errors";
import type { FieldErrors } from "../../lib/geofenceValidation";

const EVENT_TYPES: { value: AlertEventType; label: string }[] = [
  { value: "entry", label: "Entry" },
  { value: "exit", label: "Exit" },
  { value: "both", label: "Entry & exit" },
];

export function AlertRuleForm() {
  const geofences = useGeofences();
  const vehicles = useVehicles();
  const create = useCreateAlertRule();
  const [geofenceId, setGeofenceId] = useState("");
  const [vehicleId, setVehicleId] = useState(""); // "" = all vehicles
  const [eventType, setEventType] = useState<AlertEventType>("entry");
  const [errors, setErrors] = useState<FieldErrors>({});

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!geofenceId) {
      setErrors({ geofence_id: "is required" });
      return;
    }
    setErrors({});
    try {
      await create.mutateAsync({
        geofence_id: geofenceId,
        // Omitted entirely for an all-vehicles rule.
        ...(vehicleId ? { vehicle_id: vehicleId } : {}),
        event_type: eventType,
      });
      const g = geofences.data?.find((x) => x.id === geofenceId);
      const v = vehicles.data?.find((x) => x.id === vehicleId);
      toast.success(
        `Alert rule created: ${v?.vehicle_number ?? "all vehicles"} · ${eventType} · ${g?.name ?? geofenceId}`,
      );
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("An identical active rule already exists.");
      } else if (err instanceof ApiError && Object.keys(err.fieldErrors).length > 0) {
        setErrors(err.fieldErrors);
        toast.error("Please fix the highlighted fields.");
      } else if (err instanceof ApiError && err.status === 404) {
        toast.error(err.message); // geofence or vehicle was deleted meanwhile
      } else {
        toast.error(`Couldn't create rule: ${errorMessage(err)}`);
      }
    }
  };

  if (geofences.isSuccess && geofences.data.length === 0) {
    return (
      <EmptyState
        title="No geofences yet"
        description="Alert rules watch a geofence. Create one first."
        action={
          <Link to="/geofences" className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Go to Geofences →
          </Link>
        }
      />
    );
  }

  return (
    <form onSubmit={(e) => void onSubmit(e)} noValidate className="space-y-4">
      <Select
        label="Geofence"
        required
        value={geofenceId}
        disabled={geofences.isPending}
        error={errors.geofence_id}
        onChange={(e) => {
          setGeofenceId(e.target.value);
          setErrors({});
        }}
      >
        <option value="">{geofences.isPending ? "Loading…" : "Select a geofence…"}</option>
        {geofences.data?.map((g) => (
          <option key={g.id} value={g.id}>
            {g.name} — {CATEGORY_STYLES[g.category].label}
          </option>
        ))}
      </Select>
      <Select
        label="Vehicle"
        value={vehicleId}
        disabled={vehicles.isPending}
        error={errors.vehicle_id}
        hint="Leave on “All vehicles” to alert for every vehicle."
        onChange={(e) => setVehicleId(e.target.value)}
      >
        <option value="">All vehicles</option>
        {vehicles.data?.map((v) => (
          <option key={v.id} value={v.id}>
            {v.vehicle_number} · {v.driver_name}
          </option>
        ))}
      </Select>
      <fieldset>
        <legend className="mb-1 block text-sm font-medium text-slate-700">
          Trigger on<span className="ml-0.5 text-red-600">*</span>
        </legend>
        <div className="grid grid-cols-3 gap-2">
          {EVENT_TYPES.map((t) => (
            <label
              key={t.value}
              className="flex cursor-pointer items-center justify-center gap-2 rounded-md px-2 py-2 text-sm ring-1 ring-slate-300 has-[:checked]:bg-blue-50 has-[:checked]:font-medium has-[:checked]:text-blue-700 has-[:checked]:ring-blue-500 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-blue-600"
            >
              <input
                type="radio"
                name="event_type"
                value={t.value}
                checked={eventType === t.value}
                onChange={() => setEventType(t.value)}
                className="sr-only"
              />
              {t.label}
            </label>
          ))}
        </div>
      </fieldset>
      <Button type="submit" className="w-full" loading={create.isPending}>
        Create alert rule
      </Button>
    </form>
  );
}
