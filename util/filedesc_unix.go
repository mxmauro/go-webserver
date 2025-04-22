// See the LICENSE file for license details.

//go:build unix

package util

import (
	"syscall"
)

// -----------------------------------------------------------------------------

// CheckMaxFileDescriptors checks if the current process has enough file descriptors to open the given number of files.
func CheckMaxFileDescriptors(value uint64) bool {
	var rLimit syscall.Rlimit

	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	return err == nil && rLimit.Cur >= value
}
