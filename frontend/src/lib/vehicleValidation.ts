import type { CreateVehicleRequest } from "../api/types";
import type { FieldErrors } from "./geofenceValidation";

// Mirrors backend service.ValidateCreateVehicle (patterns and messages).
const VEHICLE_NUMBER = /^[A-Z0-9][A-Z0-9 -]{1,18}[A-Z0-9]$/;
const PHONE = /^\+?[0-9]{7,15}$/;
const PHONE_SEPARATORS = /[\s\-().]/g;

export const VEHICLE_TYPES = ["truck", "van", "car", "motorcycle", "bus", "pickup"] as const;

/** Upper-cases and collapses whitespace, exactly as the server stores it. */
export function normalizeVehicleNumber(raw: string): string {
  return raw.trim().split(/\s+/).filter(Boolean).join(" ").toUpperCase();
}

function requireText(errors: FieldErrors, field: string, value: string, max: number) {
  if (!value) errors[field] = "is required";
  else if ([...value].length > max) errors[field] = `must be at most ${max} characters`;
}

export function validateVehicle(values: CreateVehicleRequest): FieldErrors {
  const errors: FieldErrors = {};

  const number = normalizeVehicleNumber(values.vehicle_number);
  if (!number) errors.vehicle_number = "is required";
  else if (!VEHICLE_NUMBER.test(number)) {
    errors.vehicle_number =
      "must be 3-20 characters of letters, digits, hyphens or spaces, starting and ending with a letter or digit";
  }

  requireText(errors, "driver_name", values.driver_name.trim(), 100);
  requireText(errors, "vehicle_type", values.vehicle_type.trim(), 50);

  const phone = values.phone.trim();
  if (!phone) errors.phone = "is required";
  else if (!PHONE.test(phone.replace(PHONE_SEPARATORS, ""))) {
    errors.phone = "must be 7-15 digits with an optional leading +";
  }
  return errors;
}
