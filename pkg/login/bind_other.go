//go:build !linux

package login

import "syscall"

func createControlFunc(ifaceName string) func(string, string, syscall.RawConn) error {
	return nil
}
