import { useEffect, useId, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";

import { cn } from "../../lib/cn";
import { CloseIcon } from "./icons";

interface OverlayProps {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  children: ReactNode;
  /** Optional footer, e.g. action buttons. */
  footer?: ReactNode;
}

/**
 * Shared dialog behaviour: portal, backdrop click and Escape close, focus
 * moves into the panel on open and back to the trigger on close, page
 * scroll locked while open.
 */
const FOCUSABLE =
  "a[href], button, input:not([type=hidden]), select, textarea, [tabindex]:not([tabindex='-1'])";

function useDialog(open: boolean, onClose: () => void) {
  const panelRef = useRef<HTMLDivElement>(null);
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });

  useEffect(() => {
    if (!open) return;
    const previouslyFocused = document.activeElement as HTMLElement | null;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onCloseRef.current();
        return;
      }
      // Keep Tab / Shift+Tab inside the dialog (aria-modal requires it).
      const panelEl = panelRef.current;
      if (e.key !== "Tab" || !panelEl) return;
      const focusables = [...panelEl.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(
        (el) => !el.hasAttribute("disabled") && el.offsetParent !== null,
      );
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      if (!first || !last) return;
      const active = document.activeElement;
      if (e.shiftKey && (active === first || !panelEl.contains(active))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (active === last || !panelEl.contains(active))) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    const panel = panelRef.current;
    const firstField = panel?.querySelector<HTMLElement>(
      "input, select, textarea, button:not([data-overlay-close]), [href], [tabindex]:not([tabindex='-1'])",
    );
    (firstField ?? panel)?.focus();

    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = overflow;
      previouslyFocused?.focus();
    };
  }, [open]);

  return panelRef;
}

function Header({
  titleId,
  title,
  onClose,
}: {
  titleId: string;
  title: ReactNode;
  onClose: () => void;
}) {
  return (
    <div className="flex items-center justify-between border-b border-slate-200 px-5 py-4">
      <h2 id={titleId} className="font-semibold text-base text-slate-900">
        {title}
      </h2>
      <button
        type="button"
        data-overlay-close
        onClick={onClose}
        className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
        aria-label="Close"
      >
        <CloseIcon className="size-5" />
      </button>
    </div>
  );
}

export function Modal({ open, onClose, title, children, footer }: OverlayProps) {
  const panelRef = useDialog(open, onClose);
  const titleId = useId();
  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4">
      <div className="absolute inset-0 bg-slate-900/40" onClick={onClose} aria-hidden="true" />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="relative flex max-h-[90vh] w-full flex-col rounded-t-xl bg-white shadow-xl outline-none sm:max-w-lg sm:rounded-xl"
      >
        <Header titleId={titleId} title={title} onClose={onClose} />
        <div className="overflow-y-auto px-5 py-4">{children}</div>
        {footer && (
          <div className="flex justify-end gap-2 border-t border-slate-200 px-5 py-3">{footer}</div>
        )}
      </div>
    </div>,
    document.body,
  );
}

export function Drawer({
  open,
  onClose,
  title,
  children,
  footer,
  side = "right",
}: OverlayProps & { side?: "left" | "right" }) {
  const panelRef = useDialog(open, onClose);
  const titleId = useId();
  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-slate-900/40" onClick={onClose} aria-hidden="true" />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={cn(
          "absolute inset-y-0 flex w-full max-w-sm flex-col bg-white shadow-xl outline-none",
          side === "right" ? "right-0" : "left-0",
        )}
      >
        <Header titleId={titleId} title={title} onClose={onClose} />
        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
        {footer && (
          <div className="flex justify-end gap-2 border-t border-slate-200 px-5 py-3">{footer}</div>
        )}
      </div>
    </div>,
    document.body,
  );
}
