import { useState } from "react";
import { useLocation } from "react-router";

import { useAlertRules, useFleetLocations, useGeofences } from "../api/hooks";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge, Spinner, Tabs } from "../components/ui";
import { AlertFeed } from "../features/alerts/AlertFeed";
import { LocationMap } from "../features/locations/LocationMap";
import { LocationUpdatePanel } from "../features/locations/LocationUpdatePanel";
import { useFleetFocus } from "../features/locations/useFleetFocus";
import { useLocationDraft } from "../features/locations/useLocationDraft";
import { VehicleDetailDrawer } from "../features/vehicles/VehicleDetailDrawer";
import { useAlertSocket } from "../realtime/context";
import type { DashboardFocusState } from "./AlertsPage";

type PanelTab = "update" | "alerts";

function Stat({
  label,
  value,
  loading,
}: {
  label: string;
  value: number | undefined;
  loading: boolean;
}) {
  return (
    <div className="rounded-lg bg-white px-4 py-3 ring-1 ring-slate-200">
      <dt className="text-xs text-slate-500">{label}</dt>
      <dd className="mt-0.5 text-xl font-semibold text-slate-900">
        {loading ? <Spinner size="sm" label={`Loading ${label}`} /> : (value ?? "–")}
      </dd>
    </div>
  );
}

export function DashboardPage() {
  const geofences = useGeofences();
  const fleet = useFleetLocations();
  const rules = useAlertRules();
  const draft = useLocationDraft();
  // Opened from an alert elsewhere (e.g. the Alerts page): start focused on it.
  const initialPoint = (useLocation().state as DashboardFocusState | null)?.focusPoint ?? null;
  const [startedFocused] = useState(initialPoint !== null);
  const { focus, flyToVehicle, flyToPoint } = useFleetFocus(initialPoint);
  const [tab, setTab] = useState<PanelTab>(initialPoint ? "alerts" : "update");
  const socket = useAlertSocket();
  // Badge counts alerts that arrived live while another tab was open.
  const [seenLive, setSeenLive] = useState(socket.liveCount);
  const unseen = tab === "alerts" ? 0 : Math.max(0, socket.liveCount - seenLive);
  const [detailId, setDetailId] = useState<string | null>(null);

  const located = fleet.data?.filter((v) => v.current_location).length;
  const activeRules = rules.data?.filter((r) => r.status === "active").length;

  return (
    <>
      <PageHeader
        title="Dashboard"
        description="Live fleet map. Select a vehicle and click the map to move it."
      />

      <dl className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Geofences" value={geofences.data?.length} loading={geofences.isPending} />
        <Stat label="Vehicles" value={fleet.data?.length} loading={fleet.isPending} />
        <Stat label="Reporting location" value={located} loading={fleet.isPending} />
        <Stat label="Active alert rules" value={activeRules} loading={rules.isPending} />
      </dl>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <div className="h-[50vh] min-h-80 overflow-hidden rounded-lg ring-1 ring-slate-200 lg:h-[calc(100vh-13rem)]">
          <LocationMap
            draft={draft}
            picking={tab === "update" && draft.vehicleId !== ""}
            focus={focus}
            autoFit={!startedFocused}
          />
        </div>

        <section
          aria-label="Dashboard panel"
          className="flex flex-col gap-4 rounded-lg bg-white p-4 ring-1 ring-slate-200 lg:h-[calc(100vh-13rem)]"
        >
          <Tabs
            label="Dashboard panel"
            value={tab}
            onChange={(t) => {
              setTab(t);
              if (t === "alerts") setSeenLive(socket.liveCount);
            }}
            items={[
              { value: "update", label: "Update location" },
              {
                value: "alerts",
                label: (
                  <span className="inline-flex items-center gap-1.5">
                    Live alerts
                    {unseen > 0 && <Badge tone="red">{unseen}</Badge>}
                  </span>
                ),
              },
            ]}
          />
          <div className="min-h-0 flex-1 overflow-y-auto" role="tabpanel">
            {tab === "update" ? (
              <LocationUpdatePanel
                draft={draft}
                onVehicleSelected={flyToVehicle}
                onShowDetails={setDetailId}
              />
            ) : (
              <AlertFeed
                alerts={socket.alerts}
                loading={socket.loadingHistory}
                onSelect={(a) => flyToPoint([a.location.latitude, a.location.longitude])}
              />
            )}
          </div>
        </section>
      </div>

      <VehicleDetailDrawer vehicleId={detailId} onClose={() => setDetailId(null)} />
    </>
  );
}
