//go:build windows

package login

import (
	"context"
	"math/bits"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

const ipUnicastInterface = 31

func platformInterfaceControl(iface *net.Interface) (socketControl, error) {
	return func(_ context.Context, _, _ string, rawConn syscall.RawConn) error {
		var bindErr error
		if err := rawConn.Control(func(fd uintptr) {
			index := int(bits.ReverseBytes32(uint32(iface.Index)))
			bindErr = windows.SetsockoptInt(
				windows.Handle(fd),
				windows.IPPROTO_IP,
				ipUnicastInterface,
				index,
			)
		}); err != nil {
			return interfaceBindError(iface, err)
		}
		if bindErr != nil {
			return interfaceBindError(iface, bindErr)
		}
		return nil
	}, nil
}
