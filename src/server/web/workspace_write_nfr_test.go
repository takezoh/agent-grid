package web

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWorkspaceWriteTimingNFR101(t *testing.T) {
	if testing.Short() {
		t.Skip("timing bench skipped in -short mode")
	}
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	target := filepath.Join(root, "timing.txt")
	body := []byte(strings.Repeat("x", 64_000)) // < 100 KiB fixture
	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	pumpWorkspaceSessions(t, fd, root, 0)
	samples := make([]time.Duration, 0, 50)
	for i := 0; i < 50; i++ {
		payload := make([]byte, len(body)+1)
		copy(payload, body)
		payload[len(body)] = byte('0' + i%10)
		start := time.Now()
		req := authedPut(workspaceWriteURL("timing.txt"), payload, map[string]string{
			"If-Unmodified-Since": formatWorkspaceMtime(fi.ModTime()),
		})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		samples = append(samples, time.Since(start))
		if w.Code != http.StatusOK {
			t.Fatalf("save %d: status = %d, want 200: %s", i, w.Code, w.Body.String())
		}
		fi, err = os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p50 := samples[len(samples)*50/100]
	p95 := samples[len(samples)*95/100]
	t.Logf("save latency p50=%s p95=%s", p50, p95)
	if p50 > 500*time.Millisecond {
		t.Fatalf("p50 = %s, want <= 500ms", p50)
	}
	if p95 > 750*time.Millisecond {
		t.Fatalf("p95 = %s, want <= 750ms", p95)
	}
}

func TestWorkspaceWriteResourceBoundConcurrentRSS(t *testing.T) {
	if testing.Short() {
		t.Skip("rss bench skipped in -short mode")
	}
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	for i := 0; i < 16; i++ {
		name := filepath.Join(root, fmt.Sprintf("f%c.txt", 'a'+i))
		if err := os.WriteFile(name, []byte("seed"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	pumpWorkspaceSessions(t, fd, root, 0)
	baseline := readProcessRSS(t)
	body := make([]byte, workspaceWriteBodyCap)
	var wg sync.WaitGroup
	errCh := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			path := fmt.Sprintf("f%c.txt", 'a'+idx)
			req := authedPut(workspaceWriteURL(path), body, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				errCh <- fmt.Errorf("upload %s: status %d", path, w.Code)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		runtime.GC()
	}
	peak := readProcessRSS(t)
	delta := peak - baseline
	t.Logf("rss delta after 16 concurrent 1MiB uploads: %d bytes (baseline=%d peak=%d)", delta, baseline, peak)
	const maxDelta = 32 << 20
	if delta > maxDelta {
		t.Fatalf("rss delta = %d, want <= %d", delta, maxDelta)
	}
}

func readProcessRSS(t *testing.T) uint64 {
	t.Helper()
	runtime.GC()
	if rss, ok := linuxProcessRSS(); ok {
		return rss
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func linuxProcessRSS() (uint64, bool) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 6 || line[:6] != "VmRSS:" {
			continue
		}
		var kb uint64
		if _, err := fmt.Sscanf(line, "VmRSS: %d kB", &kb); err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}
