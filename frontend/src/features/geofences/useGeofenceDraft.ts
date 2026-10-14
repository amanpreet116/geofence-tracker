import { useCallback, useState } from "react";

import type { LatLng } from "../../api/types";
import { closeRing, isClosedRing, uniquePointCount } from "../../lib/coordinates";

export type DraftMode = "draw" | "manual";

/**
 * The polygon being created. One list of points is the source of truth for
 * both input methods: map clicks append to it, and the coordinates textarea
 * replaces it on Preview.
 */
export function useGeofenceDraft() {
  const [points, setPoints] = useState<LatLng[]>([]);
  const [mode, setMode] = useState<DraftMode>("draw");
  // Bumped when points come from the textarea, so the map fits to them.
  const [previewNonce, setPreviewNonce] = useState(0);

  const closed = isClosedRing(points);
  const canFinish = !closed && uniquePointCount(points) >= 3;

  const addPoint = useCallback((p: LatLng) => {
    setPoints((prev) => (isClosedRing(prev) ? prev : [...prev, p]));
  }, []);

  /** Removes the last vertex; on a closed ring this reopens it. */
  const undo = useCallback(() => setPoints((prev) => prev.slice(0, -1)), []);
  const clear = useCallback(() => setPoints([]), []);
  const preview = useCallback((next: LatLng[]) => {
    setPoints(next);
    setPreviewNonce((n) => n + 1);
  }, []);
  const finish = useCallback(
    () => setPoints((prev) => (uniquePointCount(prev) >= 3 ? closeRing(prev) : prev)),
    [],
  );

  return {
    points,
    mode,
    setMode,
    closed,
    canFinish,
    previewNonce,
    addPoint,
    undo,
    clear,
    finish,
    preview,
  };
}

export type GeofenceDraft = ReturnType<typeof useGeofenceDraft>;
