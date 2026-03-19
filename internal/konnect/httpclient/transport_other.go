//go:build !linux

package httpclient

import (
	"net"
	"time"
)

func configureTCPUserTimeout(_ *net.Dialer, _ time.Duration) {}
