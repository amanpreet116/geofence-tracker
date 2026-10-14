import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import { ApiError } from "../../api/client";
import { useCreateVehicle } from "../../api/hooks";
import { Button, Input, Select } from "../../components/ui";
import { errorMessage } from "../../lib/errors";
import type { FieldErrors } from "../../lib/geofenceValidation";
import {
  normalizeVehicleNumber,
  validateVehicle,
  VEHICLE_TYPES,
} from "../../lib/vehicleValidation";

const OTHER = "__other__";

export function VehicleForm({ onDone }: { onDone: (vehicleId: string) => void }) {
  const create = useCreateVehicle();
  const [vehicleNumber, setVehicleNumber] = useState("");
  const [driverName, setDriverName] = useState("");
  const [typeChoice, setTypeChoice] = useState<string>("truck");
  const [otherType, setOtherType] = useState("");
  const [phone, setPhone] = useState("");
  const [errors, setErrors] = useState<FieldErrors>({});

  const vehicleType = typeChoice === OTHER ? otherType : typeChoice;
  const clear = (field: string) =>
    setErrors((e) => Object.fromEntries(Object.entries(e).filter(([k]) => k !== field)));

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const body = {
      vehicle_number: vehicleNumber,
      driver_name: driverName.trim(),
      vehicle_type: vehicleType.trim(),
      phone: phone.trim(),
    };
    const clientErrors = validateVehicle(body);
    setErrors(clientErrors);
    if (Object.keys(clientErrors).length > 0) return;

    try {
      const res = await create.mutateAsync(body);
      toast.success(`Vehicle ${res.vehicle_number} registered`);
      onDone(res.id);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setErrors({ vehicle_number: "is already registered" });
        toast.error(err.message);
      } else if (err instanceof ApiError && Object.keys(err.fieldErrors).length > 0) {
        setErrors(err.fieldErrors);
        toast.error("Please fix the highlighted fields.");
      } else {
        toast.error(`Couldn't register vehicle: ${errorMessage(err)}`);
      }
    }
  };

  const normalized = normalizeVehicleNumber(vehicleNumber);

  return (
    <form onSubmit={(e) => void onSubmit(e)} noValidate className="space-y-4">
      <Input
        label="Vehicle number"
        required
        autoComplete="off"
        placeholder="KA-01-AB-1234"
        value={vehicleNumber}
        error={errors.vehicle_number}
        hint={
          normalized && normalized !== vehicleNumber ? `Will be saved as ${normalized}` : undefined
        }
        onChange={(e) => {
          setVehicleNumber(e.target.value);
          clear("vehicle_number");
        }}
      />
      <Input
        label="Driver name"
        required
        placeholder="John Doe"
        value={driverName}
        error={errors.driver_name}
        onChange={(e) => {
          setDriverName(e.target.value);
          clear("driver_name");
        }}
      />
      <div className="grid gap-3 sm:grid-cols-2">
        <Select
          label="Vehicle type"
          required
          value={typeChoice}
          error={typeChoice === OTHER ? undefined : errors.vehicle_type}
          onChange={(e) => {
            setTypeChoice(e.target.value);
            clear("vehicle_type");
          }}
        >
          {VEHICLE_TYPES.map((t) => (
            <option key={t} value={t}>
              {t[0]!.toUpperCase() + t.slice(1)}
            </option>
          ))}
          <option value={OTHER}>Other…</option>
        </Select>
        {typeChoice === OTHER && (
          <Input
            label="Specify type"
            required
            placeholder="e.g. tanker"
            value={otherType}
            error={errors.vehicle_type}
            onChange={(e) => {
              setOtherType(e.target.value);
              clear("vehicle_type");
            }}
          />
        )}
      </div>
      <Input
        label="Phone"
        required
        type="tel"
        autoComplete="tel"
        placeholder="+1234567890"
        value={phone}
        error={errors.phone}
        hint="7–15 digits, optional leading +; spaces and dashes are fine."
        onChange={(e) => {
          setPhone(e.target.value);
          clear("phone");
        }}
      />
      <div className="flex justify-end gap-2 border-t border-slate-200 pt-4">
        <Button type="submit" loading={create.isPending}>
          Register vehicle
        </Button>
      </div>
    </form>
  );
}
