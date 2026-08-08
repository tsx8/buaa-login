package login

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

type socketControl func(context.Context, string, string, syscall.RawConn) error

func newInterfaceControl(name string) (socketControl, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, &Error{
			Kind:      ErrorConfiguration,
			Operation: "resolve network interface",
			Message:   fmt.Sprintf("interface %q does not exist", name),
			Err:       err,
		}
	}
	return platformInterfaceControl(iface)
}

func interfaceBindError(iface *net.Interface, err error) error {
	return &Error{
		Kind:      ErrorConfiguration,
		Operation: "bind network interface",
		Message:   fmt.Sprintf("unable to bind socket to interface %q", iface.Name),
		Err:       err,
	}
}
