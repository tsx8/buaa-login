//go:build linux

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
			bindErr = unix.BindToDevice(int(fd), iface.Name)
		}); err != nil {
			return interfaceBindError(iface, err)
		}
		if bindErr != nil {
			return interfaceBindError(iface, bindErr)
		}
		return nil
	}, nil
}
