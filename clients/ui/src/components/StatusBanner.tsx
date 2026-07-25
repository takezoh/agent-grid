import { useDaemonStore } from "../store/daemon";

export function StatusBanner() {
  const daemonDisconnected = useDaemonStore((s) => s.daemonDisconnected);
  if (!daemonDisconnected) return null;
  return <output className="status-banner">daemon disconnected, reconnecting…</output>;
}
