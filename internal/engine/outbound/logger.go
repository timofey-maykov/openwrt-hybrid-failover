package outbound

import (
	"github.com/sagernet/sing/common/logger"
)

type nopLogger struct{}

func (nopLogger) Trace(args ...any)                 {}
func (nopLogger) Debug(args ...any)                 {}
func (nopLogger) Info(args ...any)                  {}
func (nopLogger) Warn(args ...any)                  {}
func (nopLogger) Error(args ...any)                 {}
func (nopLogger) Fatal(args ...any)                 {}
func (nopLogger) Panic(args ...any)                 {}

func (l nopLogger) TraceContext(ctx any, args ...any) {}
func (l nopLogger) DebugContext(ctx any, args ...any) {}
func (l nopLogger) InfoContext(ctx any, args ...any)  {}
func (l nopLogger) WarnContext(ctx any, args ...any)  {}
func (l nopLogger) ErrorContext(ctx any, args ...any) {}
func (l nopLogger) FatalContext(ctx any, args ...any) {}
func (l nopLogger) PanicContext(ctx any, args ...any) {}

var _ logger.Logger = nopLogger{}

func newNopLogger() logger.Logger { return nopLogger{} }
