package main

import (
	"os"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:]))
}
