package runtime

// HelperBinaryPath resolves a helper binary (e.g. "sockbridge") using the
// canonical exe-adjacent + libexec search implemented in runtime/rundir.go.
func (r *Runtime) HelperBinaryPath(name string) (string, error) {
	return findHelperBinary(name)
}
