// TerminalGeometry is the browser terminal's fitted character grid. The live
// xterm instance is the sole measurement owner; consumers receive snapshots
// through App instead of re-measuring the DOM independently.
export interface TerminalGeometry {
  cols: number;
  rows: number;
}
