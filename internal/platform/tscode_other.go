//go:build !windows

package platform

import (
	"io"
	"net"
)

// dialTSCode connects to the tscode center Unix domain socket.
func dialTSCode() (io.ReadWriteCloser, error) {
	return net.DialTimeout("unix", tscodePipeUnix, tscodeTimeout)
}
