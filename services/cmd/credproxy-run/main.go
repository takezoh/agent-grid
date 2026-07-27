package main

import (
	"fmt"
	"os"
)

// credproxy-run is a stub. Secret env-file resolution is handled by
// bridge secret-run (the container shim) and platform/secretenv (the host broker).
func main() {
	fmt.Fprintln(os.Stderr, "credproxy-run: use 'credproxy run --env-file X -- cmd' inside a devcontainer")
	os.Exit(1)
}
