//go:build !windows

package wslhost

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
