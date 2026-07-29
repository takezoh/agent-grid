//go:build !linux

package gitclone

func getsockoptPeerPID(int) int { return 0 }
