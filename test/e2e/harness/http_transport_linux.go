//go:build e2e && linux

package harness

import (
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func configureTCPUserTimeout(dialer *net.Dialer, timeout time.Duration) {
	if dialer == nil || timeout <= 0 {
		return
	}

	timeoutMS := int(max(timeout/time.Millisecond, 1))
	dialer.Control = func(_, _ string, rawConn syscall.RawConn) error {
		var innerErr error
		if err := rawConn.Control(func(fd uintptr) {
			innerErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, timeoutMS)
		}); err != nil {
			return err
		}
		return innerErr
	}
}
