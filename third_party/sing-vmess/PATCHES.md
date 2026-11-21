# Local changes

Based on SagerNet/sing-vmess v0.2.7, unchanged module path and upstream license.
The root and bot modules both replace that dependency with this directory.

Vision previously returned a TLS object address as uintptr from its registry
and later converted address-plus-offset back into a pointer. That loses pointer
provenance across calls, fails Go checkptr, and is not a valid way to keep an
object alive. The registry now retains unsafe.Pointer and accesses TLS buffers
with unsafe.Add. Buffer field names, direct membership and exact types are
checked before access so an incompatible TLS layout returns an error.

TLS and uTLS/REALITY registrations use the same pointer-preserving path. There
are no changes to Vision framing, certificate validation or TLS fingerprints.
No checkptr or race instrumentation is disabled.

Run the focused regression tests from the repository root:

```sh
go test -race -tags=with_utls github.com/sagernet/sing-vmess/vless ./internal/engine/outbound
```
