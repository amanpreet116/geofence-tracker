import { useRef, useState } from "react";
import { useSearchParams } from "react-router";

import { useFleetLocations, useGeofences } from "../api/hooks";
import { GEOFENCE_CATEGORIES, type GeofenceCategory } from "../api/types";
import { PageHeader } from "../components/layout/PageHeader";
import { DrawLayer } from "../components/map/DrawLayer";
import { FleetMap, type MapFocus } from "../components/map/FleetMap";
import { Button } from "../components/ui";
import { GeofenceForm } from "../features/geofences/GeofenceForm";
import { GeofenceList } from "../features/geofences/GeofenceList";
import { useGeofenceDraft } from "../features/geofences/useGeofenceDraft";
import { CATEGORY_STYLES } from "../lib/categories";

function parseCategory(value: string | null): GeofenceCategory | undefined {
  return GEOFENCE_CATEGORIES.find((c) => c === value);
}

export function GeofencesPage() {
  // The list filter lives in the URL (?category=) and is passed to the API.
  const [searchParams, setSearchParams] = useSearchParams();
  const category = parseCategory(searchParams.get("category"));
  const setCategory = (c: GeofenceCategory | undefined) =>
    setSearchParams(c ? { category: c } : {}, { replace: true });

  const geofences = useGeofences(category);
  const fleet = useFleetLocations();

  const [creating, setCreating] = useState(false);
  const [newCategory, setNewCategory] = useState<GeofenceCategory | "">("");
  const draft = useGeofenceDraft();

  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [focus, setFocus] = useState<MapFocus | null>(null);

  const mapRef = useRef<HTMLDivElement>(null);

  const select = (id: string) => {
    setSelectedId(id);
    setFocus((f) => ({ geofenceId: id, nonce: (f?.nonce ?? 0) + 1 }));
  };

  /** From the list: on narrow screens the map is above the list, so bring it into view. */
  const selectFromList = (id: string) => {
    select(id);
    if (window.matchMedia("(max-width: 1023px)").matches) {
      mapRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  };

  const startCreate = () => {
    draft.clear();
    draft.setMode("draw");
    setNewCategory("");
    setSelectedId(null);
    setCreating(true);
  };

  const endCreate = () => {
    draft.clear();
    setCreating(false);
  };

  const onCreated = (id: string) => {
    endCreate();
    // Show it even if the active filter would hide it.
    if (category && category !== newCategory) setCategory(undefined);
    select(id);
  };

  const drawColor = newCategory ? CATEGORY_STYLES[newCategory].color : "#475569";

  return (
    <>
      <PageHeader
        title="Geofences"
        description="Virtual boundaries that trigger entry and exit events."
        actions={!creating && <Button onClick={startCreate}>New geofence</Button>}
      />

      {/* Map first: on small screens it sits on top and the panel below. */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <div
          ref={mapRef}
          className="h-[45vh] min-h-72 scroll-mt-20 overflow-hidden rounded-lg ring-1 ring-slate-200 lg:sticky lg:top-20 lg:h-[calc(100vh-7rem)]"
        >
          <FleetMap
            geofences={geofences.data ?? []}
            vehicles={fleet.data}
            highlightedId={creating ? null : (hoveredId ?? selectedId)}
            focus={focus}
            onGeofenceClick={select}
            interactive={!creating}
          >
            {creating && (
              <DrawLayer
                points={draft.points}
                color={drawColor}
                active={draft.mode === "draw"}
                onAddPoint={draft.addPoint}
                onFinish={draft.finish}
                fitNonce={draft.previewNonce}
              />
            )}
          </FleetMap>
        </div>

        <section
          aria-label={creating ? "New geofence" : "Geofence list"}
          className="rounded-lg bg-white p-4 ring-1 ring-slate-200"
        >
          {creating ? (
            <>
              <h2 className="mb-4 font-semibold text-base text-slate-900">New geofence</h2>
              <GeofenceForm
                draft={draft}
                category={newCategory}
                onCategoryChange={setNewCategory}
                onCancel={endCreate}
                onCreated={onCreated}
              />
            </>
          ) : (
            <GeofenceList
              geofences={geofences.data}
              isPending={geofences.isPending}
              error={geofences.error}
              onRetry={() => void geofences.refetch()}
              category={category}
              onCategoryChange={setCategory}
              hoveredId={hoveredId}
              selectedId={selectedId}
              onHover={setHoveredId}
              onSelect={selectFromList}
              onCreate={startCreate}
            />
          )}
        </section>
      </div>
    </>
  );
}
