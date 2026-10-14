import { describe, expect, it } from "vitest";

import type { CreateVehicleRequest } from "../api/types";
import {
  diffGeofences,
  localToIsoUtc,
  toDateTimeLocal,
  validateLocation,
  type LocationFormValues,
} from "./location";
import { normalizeVehicleNumber, validateVehicle } from "./vehicleValidation";

describe("validateVehicle (mirrors backend)", () => {
  const valid: CreateVehicleRequest = {
    vehicle_number: "KA-01-AB-1234",
    driver_name: "John Doe",
    vehicle_type: "truck",
    phone: "+1234567890",
  };

  it("accepts the spec example and messy-but-valid input", () => {
    expect(validateVehicle(valid)).toEqual({});
    expect(
      validateVehicle({ ...valid, vehicle_number: "  ka 01   ab ", phone: "+44 (20) 7946-0958" }),
    ).toEqual({});
  });

  it("normalizes vehicle numbers like the server", () => {
    expect(normalizeVehicleNumber("  ka 01   ab 1234 ")).toBe("KA 01 AB 1234");
  });

  it.each<[string, Partial<CreateVehicleRequest>, string]>([
    ["number too short", { vehicle_number: "AB" }, "vehicle_number"],
    ["number bad chars", { vehicle_number: "KA_01/AB" }, "vehicle_number"],
    ["number leading hyphen", { vehicle_number: "-KA01" }, "vehicle_number"],
    ["number too long", { vehicle_number: "A".repeat(21) }, "vehicle_number"],
    ["phone too short", { phone: "123456" }, "phone"],
    ["phone too long", { phone: "+1234567890123456" }, "phone"],
    ["phone letters", { phone: "call-me" }, "phone"],
    ["driver too long", { driver_name: "x".repeat(101) }, "driver_name"],
    ["type too long", { vehicle_type: "x".repeat(51) }, "vehicle_type"],
  ])("rejects %s", (_n, patch, field) => {
    expect(Object.keys(validateVehicle({ ...valid, ...patch }))).toEqual([field]);
  });

  it("reports every missing field", () => {
    expect(
      validateVehicle({ vehicle_number: " ", driver_name: "", vehicle_type: "", phone: "" }),
    ).toEqual({
      vehicle_number: "is required",
      driver_name: "is required",
      vehicle_type: "is required",
      phone: "is required",
    });
  });
});

describe("location helpers", () => {
  const now = new Date("2025-01-15T10:35:00Z");
  const valid: LocationFormValues = {
    vehicleId: "veh_1",
    latitude: "37.7849",
    longitude: "-122.4194",
    timestamp: toDateTimeLocal(now),
  };

  it("round-trips datetime-local to ISO UTC without millis", () => {
    expect(localToIsoUtc(toDateTimeLocal(now))).toBe("2025-01-15T10:35:00Z");
    expect(localToIsoUtc("")).toBeNull();
    expect(localToIsoUtc("nonsense")).toBeNull();
  });

  it("accepts a valid update, including 0,0", () => {
    expect(validateLocation(valid, now)).toEqual({});
    expect(validateLocation({ ...valid, latitude: "0", longitude: "0" }, now)).toEqual({});
  });

  it("rejects missing, out-of-range and future values", () => {
    expect(
      validateLocation({ vehicleId: "", latitude: "", longitude: "abc", timestamp: "" }, now),
    ).toEqual({
      vehicle_id: "is required",
      latitude: "is required",
      longitude: "must be between -180 and 180",
      timestamp: "is required",
    });
    expect(validateLocation({ ...valid, latitude: "90.1" }, now)).toEqual({
      latitude: "must be between -90 and 90",
    });
    const future = toDateTimeLocal(new Date(now.getTime() + 6 * 60 * 1000));
    expect(validateLocation({ ...valid, timestamp: future }, now)).toEqual({
      timestamp: "must not be more than 5 minutes in the future",
    });
  });

  it("diffs geofence membership", () => {
    expect(diffGeofences(["a", "b"], ["b", "c"])).toEqual({ entered: ["c"], left: ["a"] });
    expect(diffGeofences([], [])).toEqual({ entered: [], left: [] });
  });
});
