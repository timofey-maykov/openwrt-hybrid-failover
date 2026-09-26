//go:build pprof

package main

// Debug builds only (go build -tags pprof): heap and goroutine profiles on
// 127.0.0.1:6060. The first process that binds the port serves them.
import (
	"net/http"
	_ "net/http/pprof"
)

func init() {
	go func() { _ = http.ListenAndServe("127.0.0.1:6060", nil) }()
}
