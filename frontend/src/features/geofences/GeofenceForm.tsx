import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import { ApiError } from "../../api/client";
import { useCreateGeofence } from "../../api/hooks";
import { GEOFENCE_CATEGORIES, type GeofenceCategory, type LatLng } from "../../api/types";
import { Button, Input, Select, Tabs, Textarea } from "../../components/ui";
import { CATEGORY_STYLES } from "../../lib/categories";
import { cn } from "../../lib/cn";
import { formatCoordinates, parseCoordinates } from "../../lib/coordinates";
import { errorMessage } from "../../lib/errors";
import { pointErrors, validateGeofence, type FieldErrors } from "../../lib/geofenceValidation";
import type { DraftMode, GeofenceDraft } from "./useGeofenceDraft";

interface Props {
  draft: GeofenceDraft;
  category: GeofenceCategory | "";
  onCategoryChange: (c: GeofenceCategory | "") => void;
  onCancel: () => void;
  onCreated: (id: string) => void;
}

/** Errors for "coordinates" and every "coordinates[i]". */
function isCoordinateKey(key: string) {
  return key === "coordinates" || key.startsWith("coordinates[");
}

export function GeofenceForm({ draft, category, onCategoryChange, onCancel, onCreated }: Props) {
  const create = useCreateGeofence();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [errors, setErrors] = useState<FieldErrors>({});
  // Textarea text while the user is editing it; null = mirror the draft points.
  const [editedText, setEditedText] = useState<string | null>(null);
  const [parseError, setParseError] = useState<string | null>(null);

  const coordinatesText = editedText ?? formatCoordinates(draft.points);

  const clearErrors = (match: (key: string) => boolean) =>
    setErrors((prev) => Object.fromEntries(Object.entries(prev).filter(([k]) => !match(k))));

  const setPointsFromInput = (points: LatLng[]) => {
    draft.preview(points);
    clearErrors(isCoordinateKey);
  };

  /** Parses the textarea into the draft; returns the points or null. */
  const applyText = (): LatLng[] | null => {
    const result = parseCoordinates(coordinatesText);
    if (!result.ok) {
      setParseError(result.error);
      return null;
    }
    setParseError(null);
    setEditedText(null);
    setPointsFromInput(result.points);
    return result.points;
  };

  const switchMode = (mode: DraftMode) => {
    if (mode === draft.mode) return;
    if (mode === "draw" && editedText !== null && applyText() === null) return;
    draft.setMode(mode);
  };

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    let coordinates = draft.points;
    if (draft.mode === "manual" && editedText !== null) {
      const parsed = applyText();
      if (!parsed) return;
      coordinates = parsed;
    }

    const values = { name, description, category, coordinates };
    const clientErrors = validateGeofence(values);
    setErrors(clientErrors);
    if (Object.keys(clientErrors).length > 0) {
      toast.error("Please fix the highlighted fields.");
      return;
    }

    try {
      const created = await create.mutateAsync({
        name: name.trim(),
        description: description.trim() || undefined,
        category: category as GeofenceCategory,
        coordinates,
      });
      toast.success(`Geofence "${created.name}" created`);
      onCreated(created.id);
    } catch (err) {
      if (err instanceof ApiError && Object.keys(err.fieldErrors).length > 0) {
        setErrors(err.fieldErrors); // server-only rules, e.g. self-intersection
        toast.error("Please fix the highlighted fields.");
      } else {
        toast.error(`Couldn't create geofence: ${errorMessage(err)}`);
      }
    }
  };

  const coordinateErrors = [errors.coordinates, ...pointErrors(errors)].filter(Boolean);

  return (
    <form onSubmit={(e) => void onSubmit(e)} noValidate className="space-y-4">
      <Input
        label="Name"
        required
        value={name}
        maxLength={200}
        onChange={(e) => {
          setName(e.target.value);
          clearErrors((k) => k === "name");
        }}
        error={errors.name}
        placeholder="Downtown Delivery Zone"
      />
      <Textarea
        label="Description"
        rows={2}
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        error={errors.description}
        placeholder="Optional"
      />
      <Select
        label="Category"
        required
        value={category}
        onChange={(e) => {
          onCategoryChange(e.target.value as GeofenceCategory | "");
          clearErrors((k) => k === "category");
        }}
        error={errors.category}
      >
        <option value="" disabled>
          Select a category…
        </option>
        {GEOFENCE_CATEGORIES.map((c) => (
          <option key={c} value={c}>
            {CATEGORY_STYLES[c].label}
          </option>
        ))}
      </Select>

      <fieldset className="space-y-3">
        <legend className="mb-1 block text-sm font-medium text-slate-700">
          Boundary<span className="ml-0.5 text-red-600">*</span>
        </legend>
        <Tabs
          label="Boundary input"
          value={draft.mode}
          onChange={switchMode}
          items={[
            { value: "draw", label: "Draw on map" },
            { value: "manual", label: "Enter coordinates" },
          ]}
        />

        {draft.mode === "draw" ? (
          <DrawControls draft={draft} />
        ) : (
          <div className="space-y-2">
            <Textarea
              label="Coordinates"
              rows={8}
              spellCheck={false}
              className="font-mono text-xs"
              value={coordinatesText}
              onChange={(e) => {
                setEditedText(e.target.value);
                setParseError(null);
              }}
              error={parseError ?? undefined}
              hint={
                'JSON [[lat, lng], …] or one "lat, lng" per line. First and last point must match.'
              }
              placeholder={
                "37.7749, -122.4194\n37.7849, -122.4194\n37.7849, -122.4094\n37.7749, -122.4094\n37.7749, -122.4194"
              }
            />
            <Button variant="secondary" size="sm" onClick={() => applyText()}>
              Preview on map
            </Button>
          </div>
        )}

        {coordinateErrors.length > 0 && (
          <ul
            role="alert"
            className="space-y-0.5 rounded-md bg-red-50 px-3 py-2 text-xs text-red-700"
          >
            {coordinateErrors.map((msg) => (
              <li key={msg}>{msg?.startsWith("Point ") ? msg : `Coordinates: ${msg}`}</li>
            ))}
          </ul>
        )}
      </fieldset>

      <div className="flex justify-end gap-2 border-t border-slate-200 pt-4">
        <Button variant="secondary" onClick={onCancel} disabled={create.isPending}>
          Cancel
        </Button>
        <Button type="submit" loading={create.isPending}>
          Create geofence
        </Button>
      </div>
    </form>
  );
}

function DrawControls({ draft }: { draft: GeofenceDraft }) {
  const vertexCount = draft.closed ? draft.points.length - 1 : draft.points.length;
  const status = draft.closed
    ? `Polygon closed with ${vertexCount} points. Undo to keep editing.`
    : vertexCount === 0
      ? "Click on the map to place the first point."
      : vertexCount < 3
        ? `${vertexCount} point${vertexCount === 1 ? "" : "s"}. Add at least ${3 - vertexCount} more.`
        : `${vertexCount} points. Click the first point or Finish to close the polygon.`;

  return (
    <div className="space-y-2">
      <p
        aria-live="polite"
        className={cn(
          "rounded-md px-3 py-2 text-sm",
          draft.closed ? "bg-green-50 text-green-800" : "bg-blue-50 text-blue-800",
        )}
      >
        {status}
      </p>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={draft.undo}
          disabled={draft.points.length === 0}
        >
          Undo point
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={draft.clear}
          disabled={draft.points.length === 0}
        >
          Clear
        </Button>
        <Button size="sm" onClick={draft.finish} disabled={!draft.canFinish}>
          Finish
        </Button>
      </div>
    </div>
  );
}
