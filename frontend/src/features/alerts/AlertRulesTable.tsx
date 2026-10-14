import { useState } from "react";
import { toast } from "sonner";

import { ApiError } from "../../api/client";
import {
  useAlertRules,
  useDeleteAlertRule,
  useGeofences,
  useUpdateAlertRule,
  useVehicles,
} from "../../api/hooks";
import type { AlertRule } from "../../api/types";
import {
  Badge,
  Button,
  CategoryBadge,
  EmptyState,
  ErrorState,
  EventTypeBadge,
  LoadingState,
  Modal,
  Select,
} from "../../components/ui";
import { BellIcon, TrashIcon } from "../../components/ui/icons";
import { Switch } from "../../components/ui/Switch";
import { errorMessage } from "../../lib/errors";
import { formatDateTime } from "../../lib/format";

function VehicleCell({ rule }: { rule: AlertRule }) {
  // null vehicle_id/vehicle_number = rule applies to all vehicles.
  return rule.vehicle_id === null ? (
    <Badge tone="blue">All vehicles</Badge>
  ) : (
    <span className="font-medium whitespace-nowrap text-slate-800">{rule.vehicle_number}</span>
  );
}

export function AlertRulesTable() {
  const [geofenceId, setGeofenceId] = useState("");
  const [vehicleId, setVehicleId] = useState("");
  const rules = useAlertRules({
    geofence_id: geofenceId || undefined,
    vehicle_id: vehicleId || undefined,
  });
  const { data: geofences } = useGeofences();
  const { data: vehicles } = useVehicles();
  const update = useUpdateAlertRule();
  const remove = useDeleteAlertRule();
  const [confirming, setConfirming] = useState<AlertRule | null>(null);

  const categoryOf = (id: string) => geofences?.find((g) => g.id === id)?.category;

  const toggle = async (rule: AlertRule, active: boolean) => {
    try {
      await update.mutateAsync({ alertId: rule.alert_id, status: active ? "active" : "inactive" });
      toast.success(`Rule ${active ? "activated" : "paused"}`);
    } catch (err) {
      toast.error(
        err instanceof ApiError && err.status === 409
          ? "An identical rule is already active; pause or delete it first."
          : `Couldn't update rule: ${errorMessage(err)}`,
      );
    }
  };

  const confirmDelete = async () => {
    if (!confirming) return;
    try {
      await remove.mutateAsync(confirming.alert_id);
      toast.success("Alert rule deleted");
      setConfirming(null);
    } catch (err) {
      toast.error(`Couldn't delete rule: ${errorMessage(err)}`);
    }
  };

  const rowActions = (r: AlertRule) => (
    <div className="flex items-center gap-3">
      <Switch
        checked={r.status === "active"}
        label={`Rule active for ${r.vehicle_number ?? "all vehicles"} on ${r.geofence_name}`}
        disabled={update.isPending && update.variables.alertId === r.alert_id}
        onChange={(on) => void toggle(r, on)}
      />
      <Button
        variant="ghost"
        size="sm"
        aria-label={`Delete rule for ${r.vehicle_number ?? "all vehicles"} on ${r.geofence_name}`}
        onClick={() => setConfirming(r)}
      >
        <TrashIcon className="size-4" />
      </Button>
    </div>
  );

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2">
        <Select label="Geofence" value={geofenceId} onChange={(e) => setGeofenceId(e.target.value)}>
          <option value="">All geofences</option>
          {geofences?.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </Select>
        <Select
          label="Vehicle"
          value={vehicleId}
          onChange={(e) => setVehicleId(e.target.value)}
          hint={vehicleId ? "Shows rules for this vehicle only, not all-vehicle rules." : undefined}
        >
          <option value="">Any vehicle</option>
          {vehicles?.map((v) => (
            <option key={v.id} value={v.id}>
              {v.vehicle_number}
            </option>
          ))}
        </Select>
      </div>

      {rules.isPending ? (
        <LoadingState label="Loading rules…" />
      ) : rules.isError ? (
        <ErrorState error={rules.error} onRetry={() => void rules.refetch()} />
      ) : rules.data.length === 0 ? (
        <EmptyState
          icon={<BellIcon />}
          title={geofenceId || vehicleId ? "No rules match these filters" : "No alert rules yet"}
          description="Create a rule to get notified when vehicles enter or leave a geofence."
        />
      ) : (
        <>
          <div className="hidden overflow-x-auto rounded-lg bg-white ring-1 ring-slate-200 md:block">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50 text-left text-xs font-medium tracking-wide text-slate-500 uppercase">
                <tr>
                  <th className="px-4 py-3">Geofence</th>
                  <th className="px-4 py-3">Vehicle</th>
                  <th className="px-4 py-3">Trigger</th>
                  <th className="px-4 py-3">Created</th>
                  <th className="px-4 py-3">Active</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {rules.data.map((r) => {
                  const category = categoryOf(r.geofence_id);
                  return (
                    <tr
                      key={r.alert_id}
                      className={r.status === "inactive" ? "opacity-60" : undefined}
                    >
                      <td className="px-4 py-3">
                        <div className="font-medium text-slate-900">{r.geofence_name}</div>
                        {category && <CategoryBadge category={category} />}
                      </td>
                      <td className="px-4 py-3">
                        <VehicleCell rule={r} />
                      </td>
                      <td className="px-4 py-3">
                        <EventTypeBadge type={r.event_type} />
                      </td>
                      <td className="px-4 py-3 text-xs text-slate-500">
                        {formatDateTime(r.created_at)}
                      </td>
                      <td className="px-4 py-3">{rowActions(r)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <ul className="space-y-2 md:hidden">
            {rules.data.map((r) => {
              const category = categoryOf(r.geofence_id);
              return (
                <li
                  key={r.alert_id}
                  className={`space-y-2 rounded-lg bg-white p-3 ring-1 ring-slate-200 ${r.status === "inactive" ? "opacity-60" : ""}`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <div className="font-medium text-slate-900">{r.geofence_name}</div>
                      {category && <CategoryBadge category={category} />}
                    </div>
                    {rowActions(r)}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <VehicleCell rule={r} />
                    <EventTypeBadge type={r.event_type} />
                  </div>
                </li>
              );
            })}
          </ul>
        </>
      )}

      <Modal
        open={confirming !== null}
        onClose={() => setConfirming(null)}
        title="Delete alert rule?"
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirming(null)}>
              Cancel
            </Button>
            <Button
              variant="danger"
              loading={remove.isPending}
              onClick={() => void confirmDelete()}
            >
              Delete rule
            </Button>
          </>
        }
      >
        {confirming && (
          <p className="text-sm text-slate-600">
            Alerts for <strong>{confirming.vehicle_number ?? "all vehicles"}</strong> on{" "}
            <strong>{confirming.geofence_name}</strong> ({confirming.event_type}) will stop. Past
            alerts stay in the history. To stop alerts temporarily, switch the rule off instead.
          </p>
        )}
      </Modal>
    </div>
  );
}
