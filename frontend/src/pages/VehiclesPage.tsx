import { useRef, useState } from "react";

import { PageHeader } from "../components/layout/PageHeader";
import { Button, Modal } from "../components/ui";
import { LocationMap } from "../features/locations/LocationMap";
import { LocationUpdatePanel } from "../features/locations/LocationUpdatePanel";
import { useFleetFocus } from "../features/locations/useFleetFocus";
import { useLocationDraft } from "../features/locations/useLocationDraft";
import { VehicleDetailDrawer } from "../features/vehicles/VehicleDetailDrawer";
import { VehicleForm } from "../features/vehicles/VehicleForm";
import { VehicleTable } from "../features/vehicles/VehicleTable";

export function VehiclesPage() {
  const draft = useLocationDraft();
  const { focus, flyToVehicle } = useFleetFocus();
  const [registering, setRegistering] = useState(false);
  const [detailId, setDetailId] = useState<string | null>(null);
  const panelRef = useRef<HTMLElement>(null);

  /** From the table or drawer: pick the vehicle in the panel and show it. */
  const startUpdate = (vehicleId: string) => {
    setDetailId(null);
    draft.setVehicleId(vehicleId);
    draft.resetPoint();
    flyToVehicle(vehicleId);
    panelRef.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  };

  return (
    <>
      <PageHeader
        title="Vehicles"
        description="Register vehicles, see where they are, and update their location."
        actions={<Button onClick={() => setRegistering(true)}>Register vehicle</Button>}
      />

      <div className="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <div className="h-[45vh] min-h-72 overflow-hidden rounded-lg ring-1 ring-slate-200 lg:h-[34rem]">
          <LocationMap draft={draft} picking={draft.vehicleId !== ""} focus={focus} />
        </div>
        <section
          ref={panelRef}
          aria-label="Update location"
          className="scroll-mt-20 rounded-lg bg-white p-4 ring-1 ring-slate-200"
        >
          <h2 className="mb-4 font-semibold text-base text-slate-900">Update location</h2>
          <LocationUpdatePanel
            draft={draft}
            onVehicleSelected={flyToVehicle}
            onShowDetails={setDetailId}
          />
        </section>
      </div>

      <h2 className="mb-3 font-semibold text-base text-slate-900">Fleet</h2>
      <VehicleTable
        selectedId={draft.vehicleId}
        onUpdateLocation={startUpdate}
        onShowDetails={setDetailId}
        onRegister={() => setRegistering(true)}
      />

      <Modal open={registering} onClose={() => setRegistering(false)} title="Register vehicle">
        <VehicleForm
          onDone={(id) => {
            setRegistering(false);
            draft.setVehicleId(id); // ready to place it on the map
          }}
        />
      </Modal>

      <VehicleDetailDrawer
        vehicleId={detailId}
        onClose={() => setDetailId(null)}
        onUpdateLocation={startUpdate}
      />
    </>
  );
}
