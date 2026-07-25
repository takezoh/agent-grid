//go:build !windows

package wslhost

// NewOSACLApplier returns the portable recorder outside Windows. Production
// Windows builds use the OS ACL implementation in acl_windows.go.
func NewOSACLApplier() ACLApplier {
	return NewRecordingACLApplier()
}
