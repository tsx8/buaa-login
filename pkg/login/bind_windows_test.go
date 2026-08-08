//go:build windows

package login

import (
	"context"
	"net"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsInterfaceBindingUsesLoopback(t *testing.T) {
	loopback := activeLoopbackInterface(t)
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

	var boundIndex int
	var readErr error
	if err := rawConn.Control(func(fd uintptr) {
		boundIndex, readErr = windows.GetsockoptInt(
			windows.Handle(fd),
			windows.IPPROTO_IP,
			ipUnicastInterface,
		)
	}); err != nil {
		t.Fatal(err)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	if boundIndex != loopback.Index {
		t.Fatalf("IP_UNICAST_IF = %d, want %d", boundIndex, loopback.Index)
	}
}
