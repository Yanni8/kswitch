//go:build windows

package terminal

import "time"

// waitReadable never sees input: color queries are not supported on
// Windows consoles. SetColors does not get this far there anyway, since
// /dev/tty does not exist and the colors are simply left alone.
func waitReadable(int, time.Duration) (bool, error) {
	return false, nil
}
