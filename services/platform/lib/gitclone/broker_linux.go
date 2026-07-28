//go:build linux

package gitclone

import "syscall"

func getsockoptPeerPID(fd int) int {
	cred, err := syscall.GetsockoptUcred(fd, syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil {
		return 0
	}
	return int(cred.Pid)
}
