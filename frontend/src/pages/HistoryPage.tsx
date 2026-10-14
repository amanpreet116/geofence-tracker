import { useRef, useState } from "react";
import { useSearchParams } from "react-router";

import { useGeofences, useVehicles, useViolations } from "../api/hooks";
import type { Violation } from "../api/types";
import { PageHeader } from "../components/layout/PageHeader";
import { Button, EmptyState, ErrorState, Select, Spinner } from "../components/ui";
import { HistoryIcon } from "../components/ui/icons";
import { DateTimeFilter } from "../features/history/DateTimeFilter";
import { EventMiniMap } from "../features/history/EventMiniMap";
import { HistoryResults, HistorySkeleton } from "../features/history/HistoryResults";
import { Pagination } from "../features/history/Pagination";
import {
  PAGE_SIZES,
  readHistoryState,
  toViolationFilters,
  violationsToCsv,
  writeHistoryState,
  type HistoryState,
} from "../lib/history";

function downloadCsv(filename: string, csv: string) {
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
  const a = Object.assign(document.createElement("a"), { href: url, download: filename });
  a.click();
  URL.revokeObjectURL(url);
}

export function HistoryPage() {
  // Every filter, the page size and the page live in the URL: views are shareable.
  const [params, setParams] = useSearchParams();
  const state = readHistoryState(params);
  const update = (patch: Partial<HistoryState>, keepPage = false) =>
    setParams(
      writeHistoryState({ ...state, ...patch, page: keepPage ? (patch.page ?? state.page) : 1 }),
      {
        replace: !keepPage,
      },
    );

  // ISO UTC strings in the same format compare chronologically.
  const rangeInvalid = Boolean(state.startDate && state.endDate && state.startDate > state.endDate);

  const history = useViolations(toViolationFilters(state), { enabled: !rangeInvalid });
  const { data: vehicles } = useVehicles();
  const { data: geofences } = useGeofences();
  const [selected, setSelected] = useState<Violation | null>(null);
  const topRef = useRef<HTMLDivElement>(null);

  const hasFilters = Boolean(
    state.vehicleId || state.geofenceId || state.startDate || state.endDate,
  );
  const rows = history.data?.violations ?? [];
  const total = history.data?.total_count ?? 0;

  const goToPage = (page: number) => {
    update({ page }, true);
    topRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  return (
    <>
      <PageHeader
        title="History"
        description="Every geofence entry and exit, newest first."
        actions={
          <Button
            variant="secondary"
            disabled={rows.length === 0}
            onClick={() =>
              downloadCsv(`geofence-events-page-${state.page}.csv`, violationsToCsv(rows))
            }
          >
            Export page (CSV)
          </Button>
        }
      />

      <section
        aria-label="Filters"
        className="mb-4 grid gap-3 rounded-lg bg-white p-4 ring-1 ring-slate-200 sm:grid-cols-2 xl:grid-cols-5"
      >
        <Select
          label="Vehicle"
          value={state.vehicleId}
          onChange={(e) => update({ vehicleId: e.target.value })}
        >
          <option value="">All vehicles</option>
          {vehicles?.map((v) => (
            <option key={v.id} value={v.id}>
              {v.vehicle_number}
            </option>
          ))}
        </Select>
        <Select
          label="Geofence"
          value={state.geofenceId}
          onChange={(e) => update({ geofenceId: e.target.value })}
        >
          <option value="">All geofences</option>
          {geofences?.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </Select>
        <DateTimeFilter
          label="From"
          iso={state.startDate}
          onCommit={(startDate) => update({ startDate })}
        />
        <DateTimeFilter
          label="To"
          iso={state.endDate}
          error={rangeInvalid ? "must be after From" : undefined}
          onCommit={(endDate) => update({ endDate })}
        />
        <div className="flex items-end gap-2">
          <div className="min-w-0 flex-1">
            <Select
              label="Per page"
              value={state.limit}
              onChange={(e) => update({ limit: Number(e.target.value) as HistoryState["limit"] })}
            >
              {PAGE_SIZES.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </Select>
          </div>
          {hasFilters && (
            <Button variant="ghost" onClick={() => setParams({}, { replace: true })}>
              Reset
            </Button>
          )}
        </div>
      </section>

      <div
        ref={topRef}
        className="grid scroll-mt-20 grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_22rem]"
      >
        {/* Mini map first in the DOM so it sits above the list on small screens. */}
        <aside className="lg:sticky lg:top-20 lg:order-2 lg:self-start">
          <EventMiniMap event={selected} />
        </aside>

        <div className="min-w-0 space-y-4 lg:order-1">
          {rangeInvalid ? (
            <EmptyState
              icon={<HistoryIcon />}
              title="Check the date range"
              description="“From” must be before “To”."
            />
          ) : history.isPending ? (
            <HistorySkeleton />
          ) : history.isError ? (
            <ErrorState
              error={history.error}
              title="Couldn't load history"
              onRetry={() => void history.refetch()}
            />
          ) : rows.length === 0 ? (
            <EmptyState
              icon={<HistoryIcon />}
              title={
                total > 0
                  ? "This page is past the end"
                  : hasFilters
                    ? "No events match these filters"
                    : "No events yet"
              }
              description={
                total > 0
                  ? undefined
                  : hasFilters
                    ? "Try widening the date range or clearing a filter."
                    : "Entry and exit events appear here as vehicles move."
              }
              action={
                total > 0 ? (
                  <Button variant="secondary" onClick={() => goToPage(1)}>
                    Go to first page
                  </Button>
                ) : hasFilters ? (
                  <Button variant="secondary" onClick={() => setParams({}, { replace: true })}>
                    Clear filters
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <>
              <div className="flex items-center justify-between">
                <Pagination
                  page={state.page}
                  limit={state.limit}
                  total={total}
                  onPage={goToPage}
                  busy={history.isPlaceholderData}
                />
                {history.isFetching && <Spinner size="sm" className="ml-2 text-slate-400" />}
              </div>
              <HistoryResults
                rows={rows}
                selectedId={selected?.id ?? null}
                onSelect={setSelected}
                dimmed={history.isPlaceholderData}
              />
              <Pagination
                page={state.page}
                limit={state.limit}
                total={total}
                onPage={goToPage}
                busy={history.isPlaceholderData}
              />
            </>
          )}
        </div>
      </div>
    </>
  );
}
