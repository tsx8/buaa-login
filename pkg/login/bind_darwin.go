//go:build darwin

package login

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func platformInterfaceControl(iface *net.Interface) (socketControl, error) {
	return func(_ context.Context, _, _ string, rawConn syscall.RawConn) error {
		var bindErr error
		if err := rawConn.Control(func(fd uintptr) {
			bindErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, iface.Index)
		}); err != nil {
			return interfaceBindError(iface, err)
		}
		if bindErr != nil {
			return interfaceBindError(iface, bindErr)
		}
		return nil
	}, nil
}
