import { toast } from "sonner";

import type { AlertMessage } from "../api/types";
import { AlertToastBody } from "./AlertToastBody";
import { isCritical } from "./alerts";

/** Restricted-zone entries are red and stay longer; other events are informational. */
export function showAlertToast(a: AlertMessage) {
  const verb = a.event_type === "entry" ? "entered" : "left";
  const title = `${a.vehicle.vehicle_number} ${verb} ${a.geofence.geofence_name}`;
  const options = { id: a.event_id, description: <AlertToastBody alert={a} /> };
  if (isCritical(a)) toast.error(title, { ...options, duration: 10_000 });
  else if (a.geofence.category === "restricted_zone") toast.warning(title, options);
  else toast.info(title, options);
}
