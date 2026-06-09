//go:build e2e && !linux

package harness

import (
	"net"
	"time"
)

func configureTCPUserTimeout(_ *net.Dialer, _ time.Duration) {}
