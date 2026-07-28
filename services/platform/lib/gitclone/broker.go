package gitclone

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type Broker struct {
	dir, sockPath   string
	nonce           []byte
	cred            Credential
	ln              net.Listener
	gitPID, gitPGID int32
	consumed        atomic.Bool
	mu              sync.Mutex
	lastCode        string
}
type BrokerPrepare struct {
	Env                    []string
	ExtraFiles             []*os.File
	Askpass, SockPath, Dir string
	Cleanup                func()
}

func StartBroker(cred Credential) (*Broker, *BrokerPrepare, error) {
	if cred.Token == "" {
		return nil, nil, errors.New(CodeAuthorityUnavailable)
	}
	dir, err := os.MkdirTemp("", "ag-clone-askpass-*")
	if err != nil {
		return nil, nil, err
	}
	fail := func(e error) (*Broker, *BrokerPrepare, error) { _ = os.RemoveAll(dir); return nil, nil, e }
	if err := os.Chmod(dir, 0o700); err != nil {
		return fail(err)
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return fail(err)
	}
	sockPath := filepath.Join(dir, "askpass.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fail(err)
	}
	if err := os.Chmod(sockPath, 0o600); err != nil {
		_ = ln.Close()
		return fail(err)
	}
	self, err := os.Executable()
	if err != nil {
		_ = ln.Close()
		return fail(err)
	}
	askpass := filepath.Join(dir, "askpass")
	script := fmt.Sprintf("#!/bin/sh\nexport %s=1\nexec %q internal-git-askpass \"$@\"\n", EnvAskpassMode, self)
	if err := os.WriteFile(askpass, []byte(script), 0o700); err != nil {
		_ = ln.Close()
		return fail(err)
	}
	rNonce, wNonce, err := os.Pipe()
	if err != nil {
		_ = ln.Close()
		return fail(err)
	}
	if _, err := wNonce.Write([]byte(hex.EncodeToString(nonce) + "\n")); err != nil {
		_ = rNonce.Close()
		_ = wNonce.Close()
		_ = ln.Close()
		return fail(err)
	}
	_ = wNonce.Close()
	b := &Broker{dir: dir, sockPath: sockPath, nonce: nonce, cred: cred, ln: ln}
	go b.serve()
	return b, &BrokerPrepare{
		Env: []string{EnvAskpassSock + "=" + sockPath, "GIT_ASKPASS=" + askpass,
			"GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"},
		ExtraFiles: []*os.File{rNonce}, Askpass: askpass, SockPath: sockPath, Dir: dir,
		Cleanup: func() { b.Close(); _ = rNonce.Close() },
	}, nil
}
func (b *Broker) BindGit(pid, pgid int) {
	atomic.StoreInt32(&b.gitPID, int32(pid))
	atomic.StoreInt32(&b.gitPGID, int32(pgid))
}
func (b *Broker) LastCode() string { b.mu.Lock(); defer b.mu.Unlock(); return b.lastCode }
func (b *Broker) Close() {
	if b.ln != nil {
		_ = b.ln.Close()
	}
	b.cred = Credential{}
	if b.dir != "" {
		_ = os.RemoveAll(b.dir)
	}
}
func (b *Broker) setCode(c string) { b.mu.Lock(); b.lastCode = c; b.mu.Unlock() }
func (b *Broker) serve() {
	for {
		c, err := b.ln.Accept()
		if err != nil {
			return
		}
		go b.handle(c)
	}
}
func (b *Broker) handle(c net.Conn) {
	defer c.Close()
	uc, ok := c.(*net.UnixConn)
	if !ok || !b.peerOK(peerPID(uc)) {
		b.setCode(CodePeerMismatch)
		return
	}
	if b.consumed.Load() {
		b.setCode(CodeReplay)
		return
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		b.setCode("malformed")
		return
	}
	got, err := hex.DecodeString(strings.TrimSpace(line))
	if err != nil || subtle.ConstantTimeCompare(got, b.nonce) != 1 {
		b.setCode("nonce_mismatch")
		return
	}
	if !b.consumed.CompareAndSwap(false, true) {
		b.setCode(CodeReplay)
		return
	}
	_, _ = io.WriteString(c, b.cred.Username+"\n"+b.cred.Token+"\n")
	b.cred = Credential{}
	b.setCode(CodeOK)
}
func (b *Broker) peerOK(pid int) bool {
	if pid <= 0 {
		return false
	}
	gitPID, gitPGID := int(atomic.LoadInt32(&b.gitPID)), int(atomic.LoadInt32(&b.gitPGID))
	if gitPID == 0 {
		return false
	}
	if pid == gitPID || (gitPGID > 0 && processPGID(pid) == gitPGID) {
		return true
	}
	pp := processPPID(pid)
	return pp == gitPID || (pp > 1 && processPPID(pp) == gitPID)
}
func RunAskpassClient(args []string) int {
	sock, nf := os.Getenv(EnvAskpassSock), os.NewFile(uintptr(AskpassNonceFD), "nonce")
	if sock == "" || nf == nil {
		return 2
	}
	nonceLine, err := bufio.NewReader(nf).ReadString('\n')
	_ = nf.Close()
	if err != nil {
		return 2
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return 2
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, nonceLine); err != nil {
		return 2
	}
	br := bufio.NewReader(conn)
	user, e1 := br.ReadString('\n')
	pass, e2 := br.ReadString('\n')
	if e1 != nil || e2 != nil {
		return 2
	}
	out := pass
	if strings.Contains(strings.ToLower(strings.Join(args, " ")), "username") {
		out = user
	}
	fmt.Print(strings.TrimRight(out, "\n"))
	return 0
}
func peerPID(conn *net.UnixConn) int {
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0
	}
	var pid int
	_ = raw.Control(func(fd uintptr) { pid = getsockoptPeerPID(int(fd)) })
	return pid
}
func processPPID(pid int) int { return procStatField(pid, 1) }
func processPGID(pid int) int { return procStatField(pid, 2) }
func procStatField(pid, idx int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	s, i := string(data), strings.LastIndex(string(data), ")")
	if i < 0 {
		return 0
	}
	fields := strings.Fields(s[i+2:])
	if len(fields) <= idx {
		return 0
	}
	n, _ := strconv.Atoi(fields[idx])
	return n
}
