import { useState } from "react";

import { Input } from "../../components/ui";
import { localToIsoUtc, toDateTimeLocal } from "../../lib/location";

const toLocal = (iso: string) => (iso ? toDateTimeLocal(new Date(iso)) : "");

/**
 * datetime-local input bound to an ISO UTC value held elsewhere (the URL).
 * Keeps the user's own text locally and only adopts `iso` when it denotes a
 * different instant (Reset, back/forward, a shared link). Re-deriving the
 * text from the URL on every change would reformat what the user just typed
 * (seconds, time-zone round trip) and fight the input.
 */
export function DateTimeFilter({
  label,
  iso,
  onCommit,
  error,
}: {
  label: string;
  iso: string;
  onCommit: (iso: string) => void;
  error?: string;
}) {
  const [text, setText] = useState(() => toLocal(iso));
  const [seenIso, setSeenIso] = useState(iso);
  if (iso !== seenIso) {
    // Adjust state while rendering (React's recommended alternative to an effect).
    setSeenIso(iso);
    if ((localToIsoUtc(text) ?? "") !== iso) setText(toLocal(iso));
  }

  return (
    <Input
      label={label}
      type="datetime-local"
      step={1}
      value={text}
      error={error}
      onChange={(e) => {
        setText(e.target.value);
        onCommit(localToIsoUtc(e.target.value) ?? "");
      }}
    />
  );
}
