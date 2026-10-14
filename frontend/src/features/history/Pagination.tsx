import { Button } from "../../components/ui";
import { cn } from "../../lib/cn";
import { pageList, pageRange } from "../../lib/history";

export function Pagination({
  page,
  limit,
  total,
  onPage,
  busy,
}: {
  page: number;
  limit: number;
  total: number;
  onPage: (page: number) => void;
  busy?: boolean;
}) {
  const { pages, from, to } = pageRange(page, limit, total);
  return (
    <nav
      aria-label="Pagination"
      className="flex flex-col items-center justify-between gap-3 sm:flex-row"
    >
      <p className="text-sm text-slate-600" aria-live="polite">
        {total === 0 ? (
          "No events"
        ) : (
          <>
            Showing <span className="font-medium">{from.toLocaleString()}</span>–
            <span className="font-medium">{to.toLocaleString()}</span> of{" "}
            <span className="font-medium">{total.toLocaleString()}</span>
          </>
        )}
      </p>
      {pages > 1 && (
        <div className="flex items-center gap-1">
          <Button
            variant="secondary"
            size="sm"
            disabled={page <= 1 || busy}
            onClick={() => onPage(page - 1)}
          >
            Previous
          </Button>
          <span className="hidden items-center gap-1 sm:flex">
            {pageList(page, pages).map((p, i) =>
              p === null ? (
                <span key={`gap-${i}`} className="px-1 text-slate-400">
                  …
                </span>
              ) : (
                <button
                  key={p}
                  type="button"
                  onClick={() => onPage(p)}
                  disabled={busy}
                  aria-current={p === page ? "page" : undefined}
                  className={cn(
                    "h-8 min-w-8 rounded-md px-2 text-sm font-medium",
                    p === page
                      ? "bg-blue-600 text-white"
                      : "text-slate-700 ring-1 ring-slate-300 hover:bg-slate-50",
                  )}
                >
                  {p}
                </button>
              ),
            )}
          </span>
          <span className="px-2 text-sm text-slate-600 sm:hidden">
            {page} / {pages}
          </span>
          <Button
            variant="secondary"
            size="sm"
            disabled={page >= pages || busy}
            onClick={() => onPage(page + 1)}
          >
            Next
          </Button>
        </div>
      )}
    </nav>
  );
}
