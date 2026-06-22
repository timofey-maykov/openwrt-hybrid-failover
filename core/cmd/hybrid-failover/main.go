package main

import (
	"os"

	_ "github.com/tmaykov/openwrt-hybrid-failover/internal/runtime/memlimit"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:]))
}
