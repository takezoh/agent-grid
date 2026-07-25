//go:build windows

package wslhost

import (
	"fmt"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowsACLApplier applies protected DACLs with the inbox icacls utility.
// Argument-vector execution avoids cmd.exe parsing.
type WindowsACLApplier struct{}

func NewOSACLApplier() ACLApplier { return WindowsACLApplier{} }

func (WindowsACLApplier) ApplyConfigACL(path, serviceIdentity string) error {
	return applyWindowsACL(path, serviceIdentity, false)
}

func (WindowsACLApplier) ApplyLogACL(path, serviceIdentity string) error {
	return applyWindowsACL(path, serviceIdentity, true)
}

func applyWindowsACL(path, serviceIdentity string, usersRead bool) error {
	if out, err := exec.Command("icacls.exe", path, "/reset").CombinedOutput(); err != nil {
		return fmt.Errorf("wslhost: icacls reset: %w: %s", err, out)
	}
	args := []string{
		path, "/inheritance:r", "/grant:r",
		"SYSTEM:(F)",
		"BUILTIN\\Administrators:(F)",
		serviceIdentity + ":(M)",
	}
	if usersRead {
		args = append(args, "BUILTIN\\Users:(R)")
	}
	if out, err := exec.Command("icacls.exe", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("wslhost: icacls apply: %w: %s", err, out)
	}
	return nil
}

func (WindowsACLApplier) ValidateConfigACL(path, serviceIdentity string) error {
	return verifyWindowsACL(path, serviceIdentity, false)
}

func (WindowsACLApplier) ValidateLogACL(path, serviceIdentity string) error {
	return verifyWindowsACL(path, serviceIdentity, true)
}

func verifyWindowsACL(path, serviceIdentity string, usersRead bool) error {
	if out, err := exec.Command("icacls.exe", path, "/verify").CombinedOutput(); err != nil {
		return fmt.Errorf("wslhost: icacls verify: %w: %s", err, out)
	}
	wantAccounts := []string{"SYSTEM", "BUILTIN\\Administrators", serviceIdentity}
	if usersRead {
		wantAccounts = append(wantAccounts, "BUILTIN\\Users")
	}
	want := make(map[string]struct{}, len(wantAccounts))
	for _, account := range wantAccounts {
		sid, _, _, err := windows.LookupSID("", account)
		if err != nil {
			return fmt.Errorf("wslhost: resolve ACL principal %q: %w", account, err)
		}
		want[sid.String()] = struct{}{}
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("wslhost: read DACL: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return fmt.Errorf("wslhost: read DACL entries: %w", err)
	}
	got := make(map[string]struct{}, dacl.AceCount)
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return fmt.Errorf("wslhost: read ACE %d: %w", i, err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return fmt.Errorf("wslhost: unexpected ACE type %d", ace.Header.AceType)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		got[sid.String()] = struct{}{}
	}
	if len(got) != len(want) {
		return fmt.Errorf("wslhost: ACL principal count mismatch: got %d want %d", len(got), len(want))
	}
	for sid := range want {
		if _, ok := got[sid]; !ok {
			return fmt.Errorf("wslhost: required ACL principal %s missing", sid)
		}
	}
	return nil
}

func (WindowsACLApplier) Applied(string) []ACLEntry { return nil }
