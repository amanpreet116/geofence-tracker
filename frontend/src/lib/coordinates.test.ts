import { describe, expect, it } from "vitest";

import type { LatLng } from "../api/types";
import { closeRing, formatCoordinates, isClosedRing, parseCoordinates } from "./coordinates";
import { pointErrors, validateGeofence, type GeofenceFormValues } from "./geofenceValidation";

const square: LatLng[] = [
  [37.7749, -122.4194],
  [37.7849, -122.4194],
  [37.7849, -122.4094],
  [37.7749, -122.4094],
  [37.7749, -122.4194],
];

describe("parseCoordinates", () => {
  it("parses JSON pairs", () => {
    expect(parseCoordinates(JSON.stringify(square))).toEqual({ ok: true, points: square });
  });

  it("parses one lat,lng per line, ignoring blanks and extra spaces", () => {
    const text = "37.7749, -122.4194\n\n  37.7849,-122.4194\n37.7849 -122.4094\n";
    expect(parseCoordinates(text)).toEqual({
      ok: true,
      points: [
        [37.7749, -122.4194],
        [37.7849, -122.4194],
        [37.7849, -122.4094],
      ],
    });
  });

  it("round-trips through formatCoordinates", () => {
    expect(parseCoordinates(formatCoordinates(square))).toEqual({ ok: true, points: square });
  });

  it.each([
    ["", "at least 4"],
    ["[[1,2],[3]]", "Item 2"],
    ['[[1,"2"]]', "Item 1"],
    ["{}", "array"],
    ["[[1,2],", "Invalid JSON"],
    ["37.7, -122.4\nfoo, bar", "Line 2"],
    ["37.7, -122.4, 5", "Line 1"],
  ])("rejects %j", (text, message) => {
    const result = parseCoordinates(text);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toContain(message);
  });
});

describe("closeRing / isClosedRing", () => {
  it("appends the first point once", () => {
    const open = square.slice(0, 4);
    expect(closeRing(open)).toEqual(square);
    expect(closeRing(square)).toBe(square);
    expect(isClosedRing(square)).toBe(true);
    expect(isClosedRing(open)).toBe(false);
  });
});

describe("validateGeofence (mirrors backend rules and messages)", () => {
  const valid: GeofenceFormValues = {
    name: "Downtown",
    description: "",
    category: "delivery_zone",
    coordinates: square,
  };

  it("accepts a valid geofence", () => {
    expect(validateGeofence(valid)).toEqual({});
  });

  it("accepts boundary values ±90/±180", () => {
    const coords: LatLng[] = [
      [-90, -180],
      [90, -180],
      [90, 180],
      [-90, -180],
    ];
    expect(validateGeofence({ ...valid, coordinates: coords })).toEqual({});
  });

  it.each<[string, Partial<GeofenceFormValues>, Record<string, string>]>([
    ["missing name", { name: "  " }, { name: "is required" }],
    ["long name", { name: "é".repeat(201) }, { name: "must be at most 200 characters" }],
    ["missing category", { category: "" }, { category: "is required" }],
    ["no coordinates", { coordinates: [] }, { coordinates: "is required" }],
    [
      "open polygon",
      { coordinates: square.slice(0, 4) },
      { coordinates: "polygon must be closed: first and last points must be identical" },
    ],
    [
      "3 points",
      { coordinates: [square[0]!, square[1]!, square[0]!] },
      {
        coordinates: "must contain at least 4 points (3 unique points plus the closing point)",
      },
    ],
    [
      "2 unique points",
      {
        coordinates: [
          [1, 1],
          [2, 2],
          [2, 2],
          [1, 1],
        ],
      },
      { coordinates: "must contain at least 3 unique points" },
    ],
    [
      "out of range",
      {
        coordinates: [
          [91, 0],
          [10, 181],
          [-90.5, -180.5],
          [91, 0],
        ],
      },
      {
        "coordinates[0]": "latitude must be between -90 and 90",
        "coordinates[1]": "longitude must be between -180 and 180",
        "coordinates[2]":
          "latitude must be between -90 and 90; longitude must be between -180 and 180",
        "coordinates[3]": "latitude must be between -90 and 90",
      },
    ],
  ])("%s", (_name, patch, expected) => {
    expect(validateGeofence({ ...valid, ...patch })).toEqual(expected);
  });

  it("caps the vertex count at 1000", () => {
    const ring: LatLng[] = Array.from({ length: 1000 }, (_, i) => [(i % 80) / 10, i / 100]);
    ring.push(ring[0]!);
    expect(validateGeofence({ ...valid, coordinates: ring })).toEqual({
      coordinates: "must contain at most 1000 points",
    });
  });

  it("rejects polygons crossing the antimeridian", () => {
    const ring: LatLng[] = [
      [-17, 179],
      [-17, -179],
      [-16, -179],
      [-16, 179],
      [-17, 179],
    ];
    expect(validateGeofence({ ...valid, coordinates: ring })).toEqual({
      coordinates:
        "polygon must not cross the antimeridian (±180° longitude); split it into two geofences",
    });
  });

  it("formats point errors in order", () => {
    expect(pointErrors({ "coordinates[10]": "b", name: "x", "coordinates[2]": "a" })).toEqual([
      "Point 3: a",
      "Point 11: b",
    ]);
  });
});
