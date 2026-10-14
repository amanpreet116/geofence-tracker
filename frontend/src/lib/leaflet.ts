// Fix Leaflet's default marker icon under Vite.
//
// Leaflet derives icon URLs from the path of its CSS at runtime
// (Icon.Default._getIconUrl). After bundling, that path no longer points at
// the images, so markers render as broken images. Importing the PNGs lets
// Vite fingerprint and serve them, and we hand Leaflet the final URLs.
import L from "leaflet";
import markerIcon2x from "leaflet/dist/images/marker-icon-2x.png";
import markerIcon from "leaflet/dist/images/marker-icon.png";
import markerShadow from "leaflet/dist/images/marker-shadow.png";

delete (L.Icon.Default.prototype as unknown as { _getIconUrl?: unknown })._getIconUrl;

L.Icon.Default.mergeOptions({
  iconRetinaUrl: markerIcon2x,
  iconUrl: markerIcon,
  shadowUrl: markerShadow,
});
