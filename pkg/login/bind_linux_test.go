//go:build linux

package login

import (
	"context"
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLinuxInterfaceBindingUsesLoopback(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	var loopback *net.Interface
	for i := range interfaces {
		if interfaces[i].Flags&net.FlagLoopback != 0 && interfaces[i].Flags&net.FlagUp != 0 {
			loopback = &interfaces[i]
			break
		}
	}
	if loopback == nil {
		t.Fatal("no active loopback interface found")
	}

	control, err := newInterfaceControl(loopback.Name)
	if err != nil {
		t.Fatalf("newInterfaceControl() error = %v", err)
	}

	socket, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	rawConn, err := socket.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := control(context.Background(), "udp4", socket.LocalAddr().String(), rawConn); err != nil {
		t.Fatalf("socket control error = %v", err)
	}

	var boundName string
	var readErr error
	if err := rawConn.Control(func(fd uintptr) {
		boundName, readErr = unix.GetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE)
	}); err != nil {
		t.Fatal(err)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	if boundName != loopback.Name {
		t.Fatalf("SO_BINDTODEVICE = %q, want %q", boundName, loopback.Name)
	}
}
