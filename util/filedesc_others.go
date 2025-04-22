// See the LICENSE file for license details.

//go:build !unix

package util

// -----------------------------------------------------------------------------

// CheckMaxFileDescriptors checks if the current process has enough file descriptors to open the given number of files.
func CheckMaxFileDescriptors(_ uint64) bool {
	return true
}
