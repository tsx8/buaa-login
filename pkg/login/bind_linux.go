//go:build linux

package login

import "syscall"

func createControlFunc(ifaceName string) func(string, string, syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var opErr error
		fn := func(fd uintptr) {
			opErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, ifaceName)
		}
		if err := c.Control(fn); err != nil {
			return err
		}
		return opErr
	}
}
