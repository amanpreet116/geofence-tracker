import type { GeofenceCategory } from "../api/types";

interface CategoryStyle {
  label: string;
  /** Tailwind classes for badges. */
  badge: string;
  /** Hex color for map polygons. */
  color: string;
}

export const CATEGORY_STYLES: Record<GeofenceCategory, CategoryStyle> = {
  restricted_zone: {
    label: "Restricted zone",
    badge: "bg-red-50 text-red-700 ring-red-600/20",
    color: "#dc2626",
  },
  delivery_zone: {
    label: "Delivery zone",
    badge: "bg-blue-50 text-blue-700 ring-blue-600/20",
    color: "#2563eb",
  },
  toll_zone: {
    label: "Toll zone",
    badge: "bg-amber-50 text-amber-800 ring-amber-600/20",
    color: "#d97706",
  },
  customer_area: {
    label: "Customer area",
    badge: "bg-green-50 text-green-700 ring-green-600/20",
    color: "#16a34a",
  },
};
