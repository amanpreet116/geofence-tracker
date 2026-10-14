import { useEffect, useRef } from "react";

import { GEOFENCE_CATEGORIES, type Geofence, type GeofenceCategory } from "../../api/types";
import {
  Button,
  CategoryBadge,
  EmptyState,
  ErrorState,
  LoadingState,
  Select,
} from "../../components/ui";
import { GeofenceIcon } from "../../components/ui/icons";
import { CATEGORY_STYLES } from "../../lib/categories";
import { cn } from "../../lib/cn";
import { formatDateTime } from "../../lib/format";

interface Props {
  geofences: Geofence[] | undefined;
  isPending: boolean;
  error: unknown;
  onRetry: () => void;
  category: GeofenceCategory | undefined;
  onCategoryChange: (c: GeofenceCategory | undefined) => void;
  hoveredId: string | null;
  selectedId: string | null;
  onHover: (id: string | null) => void;
  onSelect: (id: string) => void;
  onCreate: () => void;
}

export function GeofenceList({
  geofences,
  isPending,
  error,
  onRetry,
  category,
  onCategoryChange,
  hoveredId,
  selectedId,
  onHover,
  onSelect,
  onCreate,
}: Props) {
  const itemRefs = useRef(new Map<string, HTMLButtonElement>());

  // Selecting a polygon on the map scrolls its list item into view.
  useEffect(() => {
    if (selectedId)
      itemRefs.current.get(selectedId)?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [selectedId]);

  return (
    <div className="space-y-4">
      <Select
        label="Category"
        value={category ?? ""}
        onChange={(e) =>
          onCategoryChange((e.target.value || undefined) as GeofenceCategory | undefined)
        }
      >
        <option value="">All categories</option>
        {GEOFENCE_CATEGORIES.map((c) => (
          <option key={c} value={c}>
            {CATEGORY_STYLES[c].label}
          </option>
        ))}
      </Select>

      {isPending ? (
        <LoadingState label="Loading geofences…" />
      ) : error ? (
        <ErrorState error={error} title="Couldn't load geofences" onRetry={onRetry} />
      ) : !geofences || geofences.length === 0 ? (
        category ? (
          <EmptyState
            icon={<GeofenceIcon />}
            title={`No ${CATEGORY_STYLES[category].label.toLowerCase()}s`}
            description="Try another category or create one."
            action={
              <Button variant="secondary" size="sm" onClick={() => onCategoryChange(undefined)}>
                Show all categories
              </Button>
            }
          />
        ) : (
          <EmptyState
            icon={<GeofenceIcon />}
            title="No geofences yet"
            description="Draw a boundary on the map or paste coordinates to create the first one."
            action={<Button onClick={onCreate}>New geofence</Button>}
          />
        )
      ) : (
        <>
          <p className="text-xs text-slate-500">
            {geofences.length} geofence{geofences.length === 1 ? "" : "s"} · hover to highlight,
            click to zoom
          </p>
          <ul className="space-y-2" onMouseLeave={() => onHover(null)}>
            {geofences.map((g) => (
              <li key={g.id}>
                <button
                  type="button"
                  ref={(el) => {
                    if (el) itemRefs.current.set(g.id, el);
                    else itemRefs.current.delete(g.id);
                  }}
                  onMouseEnter={() => onHover(g.id)}
                  onFocus={() => onHover(g.id)}
                  onBlur={() => onHover(null)}
                  onClick={() => onSelect(g.id)}
                  aria-pressed={selectedId === g.id}
                  className={cn(
                    "w-full rounded-lg bg-white p-3 text-left ring-1 transition",
                    selectedId === g.id
                      ? "ring-2 ring-blue-500"
                      : hoveredId === g.id
                        ? "ring-slate-400"
                        : "ring-slate-200 hover:ring-slate-300",
                  )}
                >
                  <div className="flex items-start justify-between gap-2">
                    <span className="font-medium text-slate-900">{g.name}</span>
                    <CategoryBadge category={g.category} />
                  </div>
                  {g.description && (
                    <p className="mt-1 line-clamp-2 text-sm text-slate-600">{g.description}</p>
                  )}
                  <p className="mt-1.5 text-xs text-slate-500">
                    {g.coordinates.length - 1} vertices · created {formatDateTime(g.created_at)}
                  </p>
                </button>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
