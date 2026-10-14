import { BellRing, History, LayoutDashboard, Pentagon, Truck, type LucideIcon } from "lucide-react";

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
}

export const NAV_ITEMS: NavItem[] = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/geofences", label: "Geofences", icon: Pentagon },
  { to: "/vehicles", label: "Vehicles", icon: Truck },
  { to: "/alerts", label: "Alerts", icon: BellRing },
  { to: "/history", label: "History", icon: History },
];

/** The nav item for a pathname ("/" only matches exactly). */
export function navItemFor(pathname: string): NavItem | undefined {
  return NAV_ITEMS.find((i) => (i.to === "/" ? pathname === "/" : pathname.startsWith(i.to)));
}
