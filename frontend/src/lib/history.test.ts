import { describe, expect, it } from "vitest";

import type { Violation } from "../api/types";
import {
  pageList,
  pageRange,
  readHistoryState,
  toViolationFilters,
  violationsToCsv,
  writeHistoryState,
  type HistoryState,
} from "./history";

describe("history URL state", () => {
  const empty: HistoryState = {
    vehicleId: "",
    geofenceId: "",
    startDate: "",
    endDate: "",
    limit: 50,
    page: 1,
  };

  it("defaults when empty or malformed", () => {
    const s = readHistoryState(new URLSearchParams("limit=7&page=-2&start_date=nope"));
    expect(s).toEqual(empty);
    expect(writeHistoryState(s).toString()).toBe("");
  });

  it("round-trips through search params", () => {
    const state = {
      vehicleId: "veh_1",
      geofenceId: "geo_2",
      startDate: "2025-01-01T00:00:00Z",
      endDate: "",
      limit: 100 as const,
      page: 3,
    };
    const params = writeHistoryState(state);
    expect(params.get("start_date")).toBe("2025-01-01T00:00:00Z");
    expect(params.get("limit")).toBe("100");
    expect(readHistoryState(params)).toEqual(state);
  });

  it("normalizes dates in the URL to second-precision UTC", () => {
    const s = readHistoryState(new URLSearchParams("start_date=2025-01-01T05:30:00.250%2B05:30"));
    expect(s.startDate).toBe("2025-01-01T00:00:00Z");
  });

  it("maps page to offset for the API", () => {
    expect(toViolationFilters({ ...empty, page: 3 })).toEqual({
      vehicle_id: undefined,
      geofence_id: undefined,
      start_date: undefined,
      end_date: undefined,
      limit: 50,
      offset: 100,
    });
  });
});

describe("pagination", () => {
  it.each([
    [2, 50, 245, { pages: 5, from: 51, to: 100 }],
    [5, 50, 245, { pages: 5, from: 201, to: 245 }],
    [1, 50, 0, { pages: 1, from: 0, to: 0 }],
    [1, 500, 3, { pages: 1, from: 1, to: 3 }],
  ])("page %i × %i of %i", (page, limit, total, want) => {
    expect(pageRange(page, limit, total)).toEqual(want);
  });

  it.each([
    [1, 1, [1]],
    [1, 5, [1, 2, null, 5]],
    [3, 5, [1, 2, 3, 4, 5]],
    [10, 20, [1, null, 9, 10, 11, null, 20]],
    [20, 20, [1, null, 19, 20]],
  ])("pageList(%i, %i)", (current, pages, want) => {
    expect(pageList(current, pages)).toEqual(want);
  });
});

describe("violationsToCsv", () => {
  it("writes headers and escapes commas and quotes", () => {
    const row: Violation = {
      id: "viol_1",
      vehicle_id: "veh_1",
      vehicle_number: "KA-01",
      geofence_id: "geo_1",
      geofence_name: 'Dock "A", North',
      event_type: "entry",
      latitude: 37.7849,
      longitude: -122.4194,
      timestamp: "2025-01-15T10:35:00Z",
    };
    expect(violationsToCsv([row])).toBe(
      "id,timestamp,vehicle_id,vehicle_number,geofence_id,geofence_name,event_type,latitude,longitude\r\n" +
        'viol_1,2025-01-15T10:35:00Z,veh_1,KA-01,geo_1,"Dock ""A"", North",entry,37.7849,-122.4194\r\n',
    );
  });
});
