package selfupdate

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// State is what `update status` prints while and after an update runs.
type State struct {
	State      string   `json:"state"` // idle, running, done, failed
	Step       string   `json:"step,omitempty"`
	Message    string   `json:"message,omitempty"`
	From       string   `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
	Packages   []string `json:"packages,omitempty"`
	PID        int      `json:"pid,omitempty"`
	StartedAt  int64    `json:"started_at,omitempty"`
	FinishedAt int64    `json:"finished_at,omitempty"`
	Log        []string `json:"log,omitempty"`
}

// ReadState returns the current update state; "idle" if none was recorded.
// A "running" state whose process is gone is reported as failed, so a
// crashed or killed update does not look busy forever.
func (c *Client) ReadState() State {
	st := State{State: "idle"}
	data, err := os.ReadFile(filepath.Join(c.stateDir(), "state.json"))
	if err == nil {
		_ = json.Unmarshal(data, &st)
	}
	if st.State == "running" && st.PID > 0 && !processAlive(st.PID) {
		st.State = "failed"
		if st.Message == "" {
			st.Message = "the update process ended without reporting a result"
		}
	}
	st.Log = tailLines(filepath.Join(c.stateDir(), "update.log"), 40)
	return st
}

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func tailLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines
}

// Env is the router side of an update: package manager and commands. Tests
// replace it.
type Env struct {
	// PM is "apk" or "opkg"; empty means detect.
	PM string
	// DistribArch is DISTRIB_ARCH; empty means read /etc/openwrt_release.
	DistribArch string
	// Run executes a command and returns its combined output.
	Run func(ctx context.Context, name string, args ...string) (string, error)
}

func (e *Env) run(ctx context.Context, name string, args ...string) (string, error) {
	if e.Run != nil {
		return e.Run(ctx, name, args...)
	}
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

func (e *Env) pm() string {
	if e.PM != "" {
		return e.PM
	}
	if _, err := os.Stat("/usr/bin/apk"); err == nil {
		return "apk"
	}
	if _, err := os.Stat("/bin/opkg"); err == nil {
		return "opkg"
	}
	return "opkg"
}

func (e *Env) arch() string {
	if e.DistribArch != "" {
		return e.DistribArch
	}
	data, err := os.ReadFile("/etc/openwrt_release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "DISTRIB_ARCH=") {
			return strings.Trim(strings.TrimPrefix(line, "DISTRIB_ARCH="), `'" `)
		}
	}
	return ""
}

func (e *Env) installed(ctx context.Context, pkg string) bool {
	switch e.pm() {
	case "apk":
		_, err := e.run(ctx, "apk", "info", "-e", pkg)
		return err == nil
	default:
		out, err := e.run(ctx, "opkg", "status", pkg)
		return err == nil && strings.Contains(out, "Status: install")
	}
}

// ApplyOptions select what Apply installs.
type ApplyOptions struct {
	Installed string // version running now
	Tag       string // release tag; empty means the latest release
	Force     bool   // install even if not newer
}

// Apply downloads the release packages, verifies them and installs them,
// recording each step in the state file. It runs in the foreground; see
// StartBackground for the LuCI path.
func (c *Client) Apply(ctx context.Context, env *Env, opt ApplyOptions) (err error) {
	st := State{State: "running", From: opt.Installed, PID: os.Getpid(), StartedAt: time.Now().Unix()}
	step := func(name, msg string) {
		st.Step, st.Message = name, msg
		_ = c.writeJSON("state.json", st)
		fmt.Printf("[%s] %s: %s\n", time.Now().Format("15:04:05"), name, msg)
	}
	defer func() {
		st.FinishedAt = time.Now().Unix()
		if err != nil {
			st.State = "failed"
			st.Message = err.Error()
			fmt.Printf("[%s] failed: %v\n", time.Now().Format("15:04:05"), err)
		} else {
			st.State = "done"
		}
		_ = c.writeJSON("state.json", st)
	}()

	step("release", "looking up the release")
	tag := opt.Tag
	if tag == "" {
		rel, rerr := c.LatestRelease(ctx)
		if rerr != nil {
			return fmt.Errorf("latest release: %w", rerr)
		}
		tag = rel.Tag
	}
	st.To = strings.TrimPrefix(tag, "v")
	if !opt.Force && !Newer(st.To, opt.Installed) {
		return fmt.Errorf("%s is not newer than the installed %s", st.To, opt.Installed)
	}

	step("manifest", "reading manifest.json of "+tag)
	m, err := c.Manifest(ctx, tag)
	if err != nil {
		return err
	}
	pm := env.pm()
	archs := ArchCandidates(env.arch(), m.Architectures)
	if len(archs) == 0 {
		return fmt.Errorf("the release has no packages for architecture %q", env.arch())
	}
	pkgs := SortedPackages(func(p string) bool { return env.installed(ctx, p) })
	files, err := SelectAssets(m, pm, archs, pkgs)
	if err != nil {
		return err
	}
	st.Packages = pkgs

	dir := filepath.Join(c.stateDir(), "pkgs")
	_ = os.RemoveAll(dir)
	var paths []string
	for _, p := range pkgs {
		name := files[p]
		step("download", name)
		path, derr := c.Download(ctx, tag, name, m.Packages[name], dir)
		if derr != nil {
			return derr
		}
		paths = append(paths, path)
	}

	step("install", fmt.Sprintf("%s: %s", pm, strings.Join(pkgs, " ")))
	var args []string
	switch pm {
	case "apk":
		// The release packages are not signed with the router's apk key.
		args = append([]string{"add", "--allow-untrusted", "--force-overwrite"}, paths...)
	default:
		args = append([]string{"install", "--force-overwrite"}, paths...)
	}
	if out, ierr := env.run(ctx, pm, args...); ierr != nil {
		return fmt.Errorf("%s: %v: %s", pm, ierr, strings.TrimSpace(lastLines(out, 8)))
	}

	step("restart", "migrating the config and restarting services")
	for _, cmd := range [][]string{
		{"/usr/sbin/hybrid-failover", "migrate"},
		{"sh", "-c", "rm -rf /tmp/luci-modulecache/* /tmp/luci-indexcache* 2>/dev/null; true"},
		{"/etc/init.d/rpcd", "restart"},
		{"/etc/init.d/hybrid-failover", "restart"},
	} {
		if out, rerr := env.run(ctx, cmd[0], cmd[1:]...); rerr != nil {
			fmt.Printf("  %s: %v %s\n", strings.Join(cmd, " "), rerr, strings.TrimSpace(lastLines(out, 3)))
		}
	}
	if env.installed(ctx, "hybrid-failover-bot") {
		_, _ = env.run(ctx, "/etc/init.d/hybrid-failover-bot", "restart")
	}
	_ = os.RemoveAll(dir)
	step("done", "installed "+st.To)
	return nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// ErrBusy is returned when an update is already running.
var ErrBusy = errors.New("an update is already running")

// StartBackground starts `<self> update apply ...` detached in its own
// session, so it survives the rpcd and service restarts it causes, and
// returns right away. Output goes to update.log in the state directory.
func (c *Client) StartBackground(self string, opt ApplyOptions) (int, error) {
	if st := c.ReadState(); st.State == "running" {
		return 0, ErrBusy
	}
	if err := os.MkdirAll(c.stateDir(), 0o755); err != nil {
		return 0, err
	}
	logf, err := os.Create(filepath.Join(c.stateDir(), "update.log"))
	if err != nil {
		return 0, err
	}
	defer logf.Close()
	args := []string{"update", "apply", "--foreground"}
	if opt.Tag != "" {
		args = append(args, "--tag", opt.Tag)
	}
	if opt.Force {
		args = append(args, "--force")
	}
	cmd := exec.Command(self, args...)
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = c.writeJSON("state.json", State{State: "running", Step: "start", Message: "starting", From: opt.Installed, PID: pid, StartedAt: time.Now().Unix()})
	_ = cmd.Process.Release()
	return pid, nil
}
