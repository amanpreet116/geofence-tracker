import type { AlertRuleFilters, GeofenceCategory, ViolationFilters } from "./types";

/** Query keys, hierarchical so a prefix invalidates every variant. */
export const queryKeys = {
  geofences: {
    all: ["geofences"] as const,
    list: (category?: GeofenceCategory) => ["geofences", "list", category ?? "all"] as const,
  },
  vehicles: {
    all: ["vehicles"] as const,
    list: () => ["vehicles", "list"] as const,
  },
  locations: {
    all: ["locations"] as const,
    fleet: () => ["locations", "fleet"] as const,
    vehicle: (vehicleId: string) => ["locations", "vehicle", vehicleId] as const,
  },
  alertRules: {
    all: ["alertRules"] as const,
    list: (filters: AlertRuleFilters) => ["alertRules", "list", filters] as const,
  },
  violations: {
    all: ["violations"] as const,
    list: (filters: ViolationFilters) => ["violations", "list", filters] as const,
  },
  notifications: {
    all: ["notifications"] as const,
    list: (limit: number) => ["notifications", "list", limit] as const,
  },
} as const;
