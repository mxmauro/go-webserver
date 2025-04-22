// See the LICENSE file for license details.

package util

import (
	"unsafe"
)

// -----------------------------------------------------------------------------

// UnsafeByteSlice2String converts a byte slice to a string without copying the data.
func UnsafeByteSlice2String(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// UnsafeString2ByteSlice converts a string to a byte slice without copying the data.
func UnsafeString2ByteSlice(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
