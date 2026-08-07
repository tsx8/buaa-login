//go:build !linux

package login

import (
	"errors"
	"net"
)

func platformInterfaceControl(iface *net.Interface) (socketControl, error) {
	return nil, interfaceBindError(iface, errors.New("interface binding is not supported on this platform"))
}
