import type { ReactNode } from "react";

/**
 * Page intro under the app header. The page title itself is the header's
 * <h1> (with breadcrumb); `title` names the section for screen readers.
 */
export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div
      aria-label={title}
      role="region"
      className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
    >
      <div>{description && <p className="text-sm text-muted">{description}</p>}</div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  );
}
