import L from "leaflet";
import { useMemo } from "react";
import { Marker } from "react-leaflet";

import { useOptionalAlertSocket } from "../../realtime/context";

function pulseIcon(critical: boolean) {
  return L.divIcon({
    className: "", // drop Leaflet's default white box
    html: `<span class="alert-pulse${critical ? " alert-pulse--critical" : ""}"></span>`,
    iconSize: [48, 48],
    iconAnchor: [24, 24],
  });
}

/** Expanding rings where live alerts just fired (red for restricted entries). */
export function AlertPulseLayer() {
  const socket = useOptionalAlertSocket();
  const icons = useMemo(() => ({ normal: pulseIcon(false), critical: pulseIcon(true) }), []);
  if (!socket) return null;
  return (
    <>
      {socket.pulses.map((p) => (
        <Marker
          key={p.id}
          position={p.point}
          icon={p.critical ? icons.critical : icons.normal}
          interactive={false}
          keyboard={false}
          zIndexOffset={1000}
        />
      ))}
    </>
  );
}
