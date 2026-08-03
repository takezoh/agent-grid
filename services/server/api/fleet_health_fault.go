package api

import "os"

// AG_E2E_FLEET_HEALTH_FAULT is an opt-in real-binary fidelity seam used only by
// FakeVsRealFleetHealth* to drive production handlers/state transitions for
// scripted fleet-health divergence steps. Empty in normal production runs.
//
// Supported values match FleetHealthProbeScript step names:
//   - protocol_mismatch: capabilities reports protocol-1 (host-classified mismatch)
//   - empty_ticket: /api/ws-ticket returns HTTP 200 with an empty ticket string
//   - daemon_disconnect: /ws upgrades then suppresses hello until client cancel
//
// Boundary: the env enters the same production request handlers and lifecycle
// attach path as live traffic; it does not synthesize responses in a test proxy.
const fleetHealthFaultEnv = "AG_E2E_FLEET_HEALTH_FAULT"

func fleetHealthFault() string {
	return os.Getenv(fleetHealthFaultEnv)
}
