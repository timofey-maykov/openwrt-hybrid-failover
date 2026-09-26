package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/selfupdate"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/version"
)

func updateClient() *selfupdate.Client {
	c := &selfupdate.Client{}
	if repo := os.Getenv("HF_REPO"); repo != "" {
		c.Repo = repo
	}
	return c
}

func printJSON(v any) {
	enc, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(enc))
}

// runUpdate: hybrid-failover update check|apply|status
func runUpdate(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: hybrid-failover update check|apply|status [--tag vX.Y.Z] [--force] [--foreground]")
		return 2
	}
	c := updateClient()
	switch args[0] {
	case "check":
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		res := c.Check(ctx, version.Core)
		printJSON(res)
		if res.Error != "" {
			return 1
		}
		return 0
	case "status":
		printJSON(updateStatus(c))
		return 0
	case "apply":
		fs := flag.NewFlagSet("update apply", flag.ContinueOnError)
		tag := fs.String("tag", "", "release tag, default latest")
		force := fs.Bool("force", false, "install even if not newer")
		fg := fs.Bool("foreground", false, "run here instead of detached")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		opt := selfupdate.ApplyOptions{Installed: version.Core, Tag: *tag, Force: *force}
		if !*fg {
			self, err := os.Executable()
			if err != nil {
				self = "/usr/sbin/hybrid-failover"
			}
			pid, err := c.StartBackground(self, opt)
			if err != nil {
				printJSON(map[string]any{"started": false, "error": err.Error()})
				return 1
			}
			printJSON(map[string]any{"started": true, "pid": pid})
			return 0
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		if err := c.Apply(ctx, &selfupdate.Env{}, opt); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown update command %q\n", args[0])
		return 2
	}
}

// updateStatus is what LuCI polls: installed version, last check, state.
func updateStatus(c *selfupdate.Client) map[string]any {
	out := map[string]any{
		"installed": version.Core,
		"state":     c.ReadState(),
	}
	if chk, ok := c.CachedCheck(); ok {
		// A check cached before an update still says "available"; the
		// running binary knows better.
		chk.Installed = version.Core
		chk.Available = selfupdate.Newer(chk.Latest, version.Core)
		out["check"] = chk
	}
	return out
}
