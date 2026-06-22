//go:build linux

package tproxy

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

const soOriginalDst = 80

func originalDstPlatform(conn *net.TCPConn) (originalDest, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return originalDest{}, err
	}
	var od originalDest
	var opErr error
	err = raw.Control(func(fd uintptr) {
		var addr syscall.RawSockaddrInet4
		size := uint32(unsafe.Sizeof(addr))
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			uintptr(syscall.IPPROTO_IP),
			uintptr(soOriginalDst),
			uintptr(unsafe.Pointer(&addr)),
			uintptr(unsafe.Pointer(&size)),
			0,
		)
		if errno != 0 {
			opErr = errno
			return
		}
		port := int(addr.Port>>8&0xff) | int(addr.Port&0xff)<<8
		od = originalDest{IP: net.IPv4(addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3]), Port: port}
	})
	if err != nil {
		return originalDest{}, err
	}
	if opErr != nil {
		return originalDest{}, opErr
	}
	if od.IP == nil {
		return originalDest{}, fmt.Errorf("empty original dst")
	}
	return od, nil
}
