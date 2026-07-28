package gitclone

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrDirSyncUnsupported = errors.New("atomic_publish_unsupported: directory sync")

var ErrProcessUnavailable = errors.New("process_authority_unavailable")

type FileInfo struct {
	Name  string
	IsDir bool
	Size  int64
}

type FilesystemProcess interface {
	MkdirAll(path string, perm fs.FileMode) error
	WriteFileAtomic(path string, data []byte, perm fs.FileMode) error
	SyncFile(path string) error
	SyncTree(path string) error
	SyncDir(path string) error
	Rename(oldpath, newpath string) error
	Stat(path string) (*FileInfo, error)
	ReadFile(path string) ([]byte, error)
	RemoveAll(path string) error
	KillProcess(pid int) error
}

type OSFilesystem struct{}

func (OSFilesystem) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OSFilesystem) WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ag-pub-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	cleanup = false
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (OSFilesystem) SyncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func (OSFilesystem) SyncTree(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		syncErr := f.Sync()
		_ = f.Close()
		return syncErr
	})
}

func (OSFilesystem) SyncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("%w: %v", ErrDirSyncUnsupported, err)
	}
	return nil
}

func (OSFilesystem) Rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

func (OSFilesystem) Stat(path string) (*FileInfo, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &FileInfo{Name: st.Name(), IsDir: st.IsDir(), Size: st.Size()}, nil
}

func (OSFilesystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (OSFilesystem) RemoveAll(path string) error { return os.RemoveAll(path) }

func (OSFilesystem) KillProcess(pid int) error {
	if pid <= 0 {
		return ErrProcessUnavailable
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(os.Interrupt)
}

type FakeFilesystemProcess struct {
	FailAt string
	FailErr error
	DirSyncUnsupported bool
	FinalExists map[string]bool
	Trees map[string][]byte
	Dirs map[string]bool
	Calls []string
	Killed []int
	KillErr error
}

func NewFakeFilesystemProcess() *FakeFilesystemProcess {
	return &FakeFilesystemProcess{
		Trees: map[string][]byte{},
		Dirs:  map[string]bool{},
	}
}

func (f *FakeFilesystemProcess) note(op string) error {
	f.Calls = append(f.Calls, op)
	if f.FailAt == op {
		if f.FailErr != nil {
			return f.FailErr
		}
		return fmt.Errorf("injected failure at %s", op)
	}
	return nil
}

func (f *FakeFilesystemProcess) MkdirAll(path string, _ fs.FileMode) error {
	if err := f.note("mkdir"); err != nil {
		return err
	}
	if f.Dirs == nil {
		f.Dirs = map[string]bool{}
	}
	f.Dirs[filepath.Clean(path)] = true
	return nil
}

func (f *FakeFilesystemProcess) WriteFileAtomic(path string, data []byte, _ fs.FileMode) error {
	op := "write_atomic"
	if filepath.Base(path) == MarkerFileName {
		op = "marker_write"
	}
	if err := f.note(op); err != nil {
		return err
	}
	if f.Trees == nil {
		f.Trees = map[string][]byte{}
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	f.Trees[filepath.Clean(path)] = cp
	return nil
}

func (f *FakeFilesystemProcess) SyncFile(path string) error {
	_ = path
	return f.note("marker_sync")
}

func (f *FakeFilesystemProcess) SyncTree(path string) error {
	_ = path
	return f.note("sync_tree")
}

func (f *FakeFilesystemProcess) SyncDir(path string) error {
	_ = path
	if err := f.note("parent_sync"); err != nil {
		return err
	}
	if f.DirSyncUnsupported {
		return ErrDirSyncUnsupported
	}
	return nil
}

func (f *FakeFilesystemProcess) Rename(oldpath, newpath string) error {
	if err := f.note("rename"); err != nil {
		return err
	}
	if f.Trees == nil {
		f.Trees = map[string][]byte{}
	}
	oldc, newc := filepath.Clean(oldpath), filepath.Clean(newpath)
	moved := map[string][]byte{}
	for k, v := range f.Trees {
		if k == oldc || hasPathPrefix(k, oldc) {
			rel, _ := filepath.Rel(oldc, k)
			nk := newc
			if rel != "." {
				nk = filepath.Join(newc, rel)
			}
			moved[nk] = v
			delete(f.Trees, k)
		}
	}
	for k, v := range moved {
		f.Trees[k] = v
	}
	if f.Dirs == nil {
		f.Dirs = map[string]bool{}
	}
	f.Dirs[newc] = true
	delete(f.Dirs, oldc)
	return nil
}

func (f *FakeFilesystemProcess) Stat(path string) (*FileInfo, error) {
	f.Calls = append(f.Calls, "stat")
	c := filepath.Clean(path)
	if f.FinalExists != nil && f.FinalExists[c] {
		return &FileInfo{Name: filepath.Base(c), IsDir: true}, nil
	}
	if f.Dirs != nil && f.Dirs[c] {
		return &FileInfo{Name: filepath.Base(c), IsDir: true}, nil
	}
	if f.Trees != nil {
		if b, ok := f.Trees[c]; ok {
			return &FileInfo{Name: filepath.Base(c), Size: int64(len(b))}, nil
		}
	}
	return nil, os.ErrNotExist
}

func (f *FakeFilesystemProcess) ReadFile(path string) ([]byte, error) {
	f.Calls = append(f.Calls, "read")
	if f.Trees == nil {
		return nil, os.ErrNotExist
	}
	b, ok := f.Trees[filepath.Clean(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

func (f *FakeFilesystemProcess) RemoveAll(path string) error {
	f.Calls = append(f.Calls, "remove_all")
	c := filepath.Clean(path)
	for k := range f.Trees {
		if k == c || hasPathPrefix(k, c) {
			delete(f.Trees, k)
		}
	}
	delete(f.Dirs, c)
	return nil
}

func (f *FakeFilesystemProcess) KillProcess(pid int) error {
	f.Calls = append(f.Calls, "kill")
	if f.KillErr != nil {
		return f.KillErr
	}
	if pid <= 0 {
		return ErrProcessUnavailable
	}
	f.Killed = append(f.Killed, pid)
	return nil
}

func hasPathPrefix(path, prefix string) bool {
	rel, err := filepath.Rel(prefix, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && rel != "."
}
