import { ChevronRight, PanelLeftClose, PanelLeftOpen, Pentagon } from "lucide-react";
import { Suspense, useEffect, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router";

import { cn } from "../../lib/cn";
import { useAlertSocket } from "../../realtime/context";
import { LoadingState } from "../ui";
import { Tooltip } from "../ui/Tooltip";
import { AlertsBell } from "./AlertsBell";
import { ConnectionIndicator } from "./ConnectionIndicator";
import { NAV_ITEMS, navItemFor } from "./nav";
import { ThemeToggle } from "./ThemeToggle";

const RAIL_KEY = "gt-rail-collapsed";

function useRailCollapsed() {
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(RAIL_KEY) === "1";
    } catch {
      return false;
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem(RAIL_KEY, collapsed ? "1" : "0");
    } catch {
      /* ignore */
    }
  }, [collapsed]);
  return [collapsed, setCollapsed] as const;
}

function Brand({ compact }: { compact: boolean }) {
  return (
    <div className="flex items-center gap-2.5">
      <span className="flex size-7 shrink-0 items-center justify-center rounded-control bg-accent text-accent-fg">
        <Pentagon className="size-4" strokeWidth={2.25} aria-hidden />
      </span>
      {!compact && (
        <span className="truncate text-sm font-semibold tracking-tight text-fg">Fleet console</span>
      )}
    </div>
  );
}

/** Desktop: slim collapsible left rail; tooltips label icons when collapsed. */
function Rail() {
  const [collapsed, setCollapsed] = useRailCollapsed();
  const ToggleIcon = collapsed ? PanelLeftOpen : PanelLeftClose;
  return (
    <aside
      className={cn(
        "sticky top-0 hidden h-screen shrink-0 flex-col border-r border-subtle bg-surface transition-[width] duration-200 ease-out md:flex",
        collapsed ? "w-14" : "w-52",
      )}
    >
      <div
        className={cn(
          "flex h-14 items-center border-b border-subtle",
          collapsed ? "justify-center" : "px-4",
        )}
      >
        <Brand compact={collapsed} />
      </div>
      <nav aria-label="Main" className="flex-1 space-y-0.5 p-2">
        {NAV_ITEMS.map(({ to, label, icon: Icon }) => (
          <Tooltip key={to} content={label} side="right" disabled={!collapsed}>
            <NavLink
              to={to}
              end={to === "/"}
              aria-label={collapsed ? label : undefined}
              className={({ isActive }) =>
                cn(
                  "relative flex h-9 items-center gap-3 rounded-control text-sm font-medium transition-colors",
                  collapsed ? "justify-center" : "px-3",
                  isActive ? "bg-raised text-fg" : "text-muted hover:bg-hover hover:text-fg",
                )
              }
            >
              {({ isActive }) => (
                <>
                  {isActive && (
                    <span
                      className="absolute top-1.5 bottom-1.5 left-0 w-0.5 rounded-full bg-accent"
                      aria-hidden
                    />
                  )}
                  <Icon
                    className={cn("size-4 shrink-0", isActive && "text-accent-text")}
                    aria-hidden
                  />
                  {!collapsed && label}
                </>
              )}
            </NavLink>
          </Tooltip>
        ))}
      </nav>
      <div className="border-t border-subtle p-2">
        <Tooltip content={collapsed ? "Expand sidebar" : "Collapse sidebar"} side="right">
          <button
            type="button"
            onClick={() => setCollapsed((c) => !c)}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            aria-expanded={!collapsed}
            className={cn(
              "flex h-9 w-full items-center gap-3 rounded-control text-sm text-muted hover:bg-hover hover:text-fg",
              collapsed ? "justify-center" : "px-3",
            )}
          >
            <ToggleIcon className="size-4" aria-hidden />
            {!collapsed && "Collapse"}
          </button>
        </Tooltip>
      </div>
    </aside>
  );
}

/** Mobile: bottom tab bar (56px targets, safe-area aware). */
function BottomTabs() {
  return (
    <nav
      aria-label="Main"
      className="fixed inset-x-0 bottom-0 z-40 grid grid-cols-5 border-t border-subtle bg-surface pb-[env(safe-area-inset-bottom)] md:hidden"
    >
      {NAV_ITEMS.map(({ to, label, icon: Icon }) => (
        <NavLink
          key={to}
          to={to}
          end={to === "/"}
          className={({ isActive }) =>
            cn(
              "flex h-14 flex-col items-center justify-center gap-1 text-[11px] font-medium",
              isActive ? "text-accent-text" : "text-muted",
            )
          }
        >
          <Icon className="size-5" aria-hidden />
          {label}
        </NavLink>
      ))}
    </nav>
  );
}

function Header() {
  const { pathname } = useLocation();
  const { status } = useAlertSocket();
  const current = navItemFor(pathname);
  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-subtle bg-base/95 px-4 backdrop-blur-sm md:px-6">
      <div className="md:hidden">
        <Brand compact />
      </div>
      <div className="min-w-0">
        <nav
          aria-label="Breadcrumb"
          className="hidden items-center gap-1 text-xs text-faint sm:flex"
        >
          <span>Fleet console</span>
          <ChevronRight className="size-3" aria-hidden />
          <span className="text-muted">{current?.label ?? "Not found"}</span>
        </nav>
        <h1 className="truncate leading-tight font-semibold text-base text-fg">
          {current?.label ?? "Page not found"}
        </h1>
      </div>
      <div className="ml-auto flex items-center gap-1 md:gap-2">
        <ConnectionIndicator status={status} />
        <AlertsBell />
        <ThemeToggle />
      </div>
    </header>
  );
}

export function AppShell() {
  return (
    <div className="flex min-h-full">
      <Rail />
      <div className="flex min-w-0 flex-1 flex-col">
        <Header />
        <main className="flex-1 p-4 pb-24 md:p-6 md:pb-6">
          <Suspense fallback={<LoadingState />}>
            <Outlet />
          </Suspense>
        </main>
      </div>
      <BottomTabs />
    </div>
  );
}
