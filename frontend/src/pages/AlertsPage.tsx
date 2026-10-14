import { useNavigate } from "react-router";

import type { AlertMessage } from "../api/types";
import { ConnectionIndicator } from "../components/layout/ConnectionIndicator";
import { PageHeader } from "../components/layout/PageHeader";
import { AlertFeed } from "../features/alerts/AlertFeed";
import { AlertRuleForm } from "../features/alerts/AlertRuleForm";
import { AlertRulesTable } from "../features/alerts/AlertRulesTable";
import { useAlertSocket } from "../realtime/context";

/** Router state the Dashboard reads to open focused on a point. */
export interface DashboardFocusState {
  focusPoint: [number, number];
}

export function AlertsPage() {
  const { alerts, loadingHistory, status } = useAlertSocket();
  const navigate = useNavigate();

  // This page has no map: open the Dashboard focused on the alert.
  const showOnMap = (a: AlertMessage) =>
    navigate("/", {
      state: {
        focusPoint: [a.location.latitude, a.location.longitude],
      } satisfies DashboardFocusState,
    });

  return (
    <>
      <PageHeader
        title="Alerts"
        description="Configure alert rules and watch alerts arrive in real time."
      />
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[22rem_minmax(0,1fr)]">
        <div className="space-y-6">
          <section className="rounded-lg bg-white p-4 ring-1 ring-slate-200">
            <h2 className="mb-4 font-semibold text-base text-slate-900">New alert rule</h2>
            <AlertRuleForm />
          </section>
          <section
            aria-labelledby="live-alerts"
            className="rounded-lg bg-white p-4 ring-1 ring-slate-200"
          >
            <div className="mb-4 flex items-center justify-between gap-2">
              <h2 id="live-alerts" className="font-semibold text-base text-slate-900">
                Live alerts
              </h2>
              <ConnectionIndicator status={status} />
            </div>
            <div className="max-h-[32rem] overflow-y-auto">
              <AlertFeed alerts={alerts} loading={loadingHistory} onSelect={showOnMap} />
            </div>
          </section>
        </div>
        <section className="min-w-0 rounded-lg bg-white p-4 ring-1 ring-slate-200">
          <h2 className="mb-4 font-semibold text-base text-slate-900">Alert rules</h2>
          <AlertRulesTable />
        </section>
      </div>
    </>
  );
}
