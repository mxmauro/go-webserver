// See the LICENSE file for license details.

package storage

import (
	"time"
)

// -----------------------------------------------------------------------------

// Storage interface for communicating with different database/key-value providers
type Storage interface {
	// Close shuts down the storage.
	Close()

	// Get gets the value for the given key. If the key does not exist, nil is returned.
	Get(key []byte) ([]byte, error)

	// Set stores the given value for the given key along with an expiration value, 0 means no expiration.
	Set(key []byte, val []byte, exp time.Duration) error

	// Delete deletes the value for the given key. No error is raised if the key does not exist.
	Delete(key []byte) error

	// Reset deletes all the keys stored in the storage.
	Reset() error
}
