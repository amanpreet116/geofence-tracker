import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api } from "./client";
import { queryKeys } from "./keys";
import type {
  AlertNotificationsResponse,
  AlertRuleFilters,
  AlertRuleListResponse,
  ConfigureAlertRequest,
  ConfigureAlertResponse,
  CreateGeofenceRequest,
  CreateGeofenceResponse,
  CreateVehicleRequest,
  CreateVehicleResponse,
  DeleteAlertRuleResponse,
  FleetLocationsResponse,
  FleetVehicle,
  GeofenceCategory,
  GeofenceListResponse,
  UpdateAlertRuleRequest,
  UpdateAlertRuleResponse,
  UpdateLocationRequest,
  UpdateLocationResponse,
  VehicleListResponse,
  VehicleLocationResponse,
  ViolationFilters,
  ViolationHistoryResponse,
} from "./types";

// ---- Geofences ---------------------------------------------------------------

export function useGeofences(category?: GeofenceCategory) {
  return useQuery({
    queryKey: queryKeys.geofences.list(category),
    queryFn: ({ signal }) =>
      api<GeofenceListResponse>("/geofences", { query: { category }, signal }),
    select: (data) => data.geofences,
  });
}

export function useCreateGeofence() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateGeofenceRequest) =>
      api<CreateGeofenceResponse>("/geofences", { method: "POST", body }),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.geofences.all }),
  });
}

// ---- Vehicles & locations ----------------------------------------------------

export function useVehicles() {
  return useQuery({
    queryKey: queryKeys.vehicles.list(),
    queryFn: ({ signal }) => api<VehicleListResponse>("/vehicles", { signal }),
    select: (data) => data.vehicles,
  });
}

export function useCreateVehicle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateVehicleRequest) =>
      api<CreateVehicleResponse>("/vehicles", { method: "POST", body }),
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: queryKeys.vehicles.all }),
        qc.invalidateQueries({ queryKey: queryKeys.locations.all }),
      ]),
  });
}

/** Every vehicle's last position and current geofence ids (one request). */
export function useFleetLocations() {
  return useQuery({
    queryKey: queryKeys.locations.fleet(),
    queryFn: ({ signal }) => api<FleetLocationsResponse>("/fleet/locations", { signal }),
    select: (data) => data.vehicles,
  });
}

export function useVehicleLocation(vehicleId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.locations.vehicle(vehicleId ?? ""),
    queryFn: ({ signal }) =>
      api<VehicleLocationResponse>(`/vehicles/location/${encodeURIComponent(vehicleId ?? "")}`, {
        signal,
      }),
    enabled: Boolean(vehicleId),
  });
}

/**
 * POST /vehicles/location with an optimistic marker move: the fleet cache
 * gets the new position immediately, the response's geofences are written
 * on success, and a refetch reconciles (e.g. the server ignores
 * out-of-order reports, so the marker snaps back). Rolls back on error.
 */
export function useUpdateLocation() {
  const qc = useQueryClient();
  const fleetKey = queryKeys.locations.fleet();
  const patchVehicle = (vehicleId: string, patch: (v: FleetVehicle) => FleetVehicle) =>
    qc.setQueryData<FleetLocationsResponse>(
      fleetKey,
      (old) =>
        old && {
          ...old,
          vehicles: old.vehicles.map((v) => (v.vehicle_id === vehicleId ? patch(v) : v)),
        },
    );

  return useMutation({
    mutationFn: (body: UpdateLocationRequest) =>
      api<UpdateLocationResponse>("/vehicles/location", { method: "POST", body }),
    onMutate: async (body) => {
      await qc.cancelQueries({ queryKey: fleetKey });
      const previous = qc.getQueryData<FleetLocationsResponse>(fleetKey);
      patchVehicle(body.vehicle_id, (v) => ({
        ...v,
        current_location: {
          latitude: body.latitude,
          longitude: body.longitude,
          timestamp: body.timestamp,
        },
      }));
      return { previous };
    },
    onError: (_err, _body, context) => {
      if (context?.previous) qc.setQueryData(fleetKey, context.previous);
    },
    onSuccess: (res) => {
      const ids = res.current_geofences.map((g) => g.geofence_id).sort();
      patchVehicle(res.vehicle_id, (v) => ({ ...v, current_geofence_ids: ids }));
    },
    // A move can change membership and create entry/exit events.
    onSettled: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: queryKeys.locations.all }),
        qc.invalidateQueries({ queryKey: queryKeys.violations.all }),
        qc.invalidateQueries({ queryKey: queryKeys.notifications.all }),
      ]),
  });
}

// ---- Alert rules -------------------------------------------------------------

export function useAlertRules(filters: AlertRuleFilters = {}) {
  return useQuery({
    queryKey: queryKeys.alertRules.list(filters),
    queryFn: ({ signal }) =>
      api<AlertRuleListResponse>("/alerts", { query: { ...filters }, signal }),
    select: (data) => data.alerts,
  });
}

export function useCreateAlertRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ConfigureAlertRequest) =>
      api<ConfigureAlertResponse>("/alerts/configure", { method: "POST", body }),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.alertRules.all }),
  });
}

export function useUpdateAlertRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ alertId, ...body }: UpdateAlertRuleRequest & { alertId: string }) =>
      api<UpdateAlertRuleResponse>(`/alerts/${encodeURIComponent(alertId)}`, {
        method: "PATCH",
        body,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.alertRules.all }),
  });
}

export function useDeleteAlertRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (alertId: string) =>
      api<DeleteAlertRuleResponse>(`/alerts/${encodeURIComponent(alertId)}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.alertRules.all }),
  });
}

// ---- History & notifications -------------------------------------------------

export function useViolations(filters: ViolationFilters = {}, { enabled = true } = {}) {
  return useQuery({
    queryKey: queryKeys.violations.list(filters),
    queryFn: ({ signal }) =>
      api<ViolationHistoryResponse>("/violations/history", { query: { ...filters }, signal }),
    // Keep the current page visible while the next one loads (no flicker).
    placeholderData: keepPreviousData,
    enabled,
  });
}

/** Recently dispatched alerts, to seed the live feed after a reload. */
export function useAlertNotifications(limit = 50) {
  return useQuery({
    queryKey: queryKeys.notifications.list(limit),
    queryFn: ({ signal }) =>
      api<AlertNotificationsResponse>("/alerts/notifications", { query: { limit }, signal }),
    select: (data) => data.notifications,
  });
}
