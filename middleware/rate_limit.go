// See the LICENSE file for license details.

package middleware

import (
	"encoding/binary"
	"hash/crc32"
	"strconv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/storage"
	"github.com/mxmauro/go-webserver/v2/util"
)

// -----------------------------------------------------------------------------

// KeyGeneratorFunc defines a function that generates the rate-limit key.
type KeyGeneratorFunc func(req *webserver.RequestContext) []byte

// LimitReachedHandler defines a function to call when the rate limit is reached.
type LimitReachedHandler func(req *webserver.RequestContext) error

// RateLimiterOptions defines the behavior of the rate limiter middleware.
type RateLimiterOptions struct {
	// Max number of connections during `Expiration` seconds before sending a 429 response. Defaults to 6
	Max int

	// Expiration defines the window size. Defaults to 1 minute.
	Expiration time.Duration

	// KeyGenerator allows you to generate custom keys. Defaults to req.RemoteIP().
	KeyGenerator KeyGeneratorFunc

	// LimitReached is called when a request hits the limit. Defaults to return status 429.
	LimitReached LimitReachedHandler

	// If true, requests with StatusCode >= 400 won't be counted.
	SkipFailedRequests bool

	// Store is used to store the state of the middleware. If not defined, an internal memory storage will be used.
	ExternalStorage storage.Storage

	// MaxMemoryCacheSize indicates the maximum amount of memory to use if no external storage is used.
	MaxMemoryCacheSize int
}

const rateLimiterMutexCount = 16 // NOTE: This number must be a power of two

// NOTE: rateLimiterMemoryCacheEntriesCount does not reflect the real number of entries that will fit in the
//
//	memory cache due to fastcache.Cache extra usage, but it is a good approximation.
const rateLimiterMemoryCacheEntriesCount = 100000

// sizeof(currHits) + sizeof(prevHits) + sizeof(exp) = 16
const rateLimiterItemPackedSizeInBytes = 16

type rateLimiterItem struct {
	storage storage.Storage
	cache   *fastcache.Cache
	key     []byte

	currHits uint32
	prevHits uint32
	exp      uint64
}

// -----------------------------------------------------------------------------

// NewRateLimiter wraps a middleware that limits the number of requests from the same source.
func NewRateLimiter(opts RateLimiterOptions) webserver.HandlerFunc {
	var expiration uint64
	var cache *fastcache.Cache
	var mtx [rateLimiterMutexCount]sync.Mutex

	if opts.Max <= 0 {
		opts.Max = 10
	}
	if opts.Expiration > 0 {
		expiration = uint64(opts.Expiration.Seconds())
	} else {
		expiration = 60
	}

	if opts.KeyGenerator == nil {
		opts.KeyGenerator = func(req *webserver.RequestContext) []byte {
			return req.RemoteIP()
		}
	}
	if opts.LimitReached == nil {
		opts.LimitReached = func(req *webserver.RequestContext) error {
			req.TooManyRequests("")
			return nil
		}
	}
	if opts.ExternalStorage == nil {
		size := opts.MaxMemoryCacheSize + rateLimiterItemPackedSizeInBytes - 1
		size -= size % rateLimiterItemPackedSizeInBytes
		if size < rateLimiterMemoryCacheEntriesCount*rateLimiterItemPackedSizeInBytes {
			size = rateLimiterMemoryCacheEntriesCount * rateLimiterItemPackedSizeInBytes
		}
		cache = fastcache.New(size)
	}

	// Set up the middleware function.
	return func(req *webserver.RequestContext) error {
		// Get the key from the request.
		key := opts.KeyGenerator(req)

		// Calculate the lock slot to use.
		mtxIdx := crc32.ChecksumIEEE(key) & (rateLimiterMutexCount - 1)

		// Lock the entry.
		mtx[mtxIdx].Lock()

		// Get the entry from the cache.
		entry := rateLimiterItem{
			storage: opts.ExternalStorage,
			cache:   cache,
			key:     key,
		}
		err := entry.load()
		if err != nil {
			mtx[mtxIdx].Unlock()
			return err
		}

		// Get the current timestamp.
		ts := uint64(time.Now().Unix())

		// Set the new expiration.
		entry.setExpiration(ts, expiration)

		// Increment the hit count.
		entry.currHits += 1

		// Calculate the reset time and current rate.
		resetTime := uint64(0)
		if ts < entry.exp {
			resetTime = entry.exp - ts
		}
		rate := int((uint64(entry.prevHits)*resetTime)/expiration + uint64(entry.currHits))

		// Calculate how many hits remain based on the current rate.
		remaining := opts.Max - rate

		// Save the updated information.
		err = entry.save(resetTime + expiration)
		if err != nil {
			mtx[mtxIdx].Unlock()
			return err
		}

		// Unlock the entry.
		mtx[mtxIdx].Unlock()

		// Check whether the hits exceed cfg.Max.
		if remaining < 0 {
			// Call the LimitReached handler.
			err = opts.LimitReached(req)
			if err != nil {
				return err
			}

			// Add Retry-After if it was not set by the handler.
			if len(req.Response().Header.PeekBytes(util.HeaderHeaderRetryAfter)) == 0 {
				// Return the response with the Retry-After header (https://tools.ietf.org/html/rfc6584).
				req.Response().Header.SetBytesK(util.HeaderHeaderRetryAfter, strconv.FormatUint(resetTime, 10))
			}

			// Done.
			return nil
		}

		// Call the next handler and save the error.
		err = req.Next()

		// Check SkipFailedRequests.
		if opts.SkipFailedRequests && req.Response().StatusCode() >= 400 {
			// Lock the entry.
			mtx[mtxIdx].Lock()

			err2 := entry.load()
			if err2 == nil && entry.currHits > 0 {
				entry.currHits -= 1
				remaining += 1
				err2 = entry.save(expiration)
			}

			// Unlock the entry.
			mtx[mtxIdx].Unlock()

			if err2 != nil {
				return err2
			}
		}

		// Continue and update the RateLimit headers.
		resp := req.Response()
		resp.Header.SetBytesK(util.HeaderXRateLimitLimit, strconv.Itoa(opts.Max))
		resp.Header.SetBytesK(util.HeaderXRateLimitRemaining, strconv.Itoa(remaining))
		resp.Header.SetBytesK(util.HeaderXRateLimitReset, strconv.FormatUint(resetTime, 10))

		// Done.
		return err
	}
}

// -----------------------------------------------------------------------------

func (entry *rateLimiterItem) encode(dst []byte) {
	binary.LittleEndian.PutUint32(dst, entry.currHits)
	binary.LittleEndian.PutUint32(dst[4:], entry.prevHits)
	binary.LittleEndian.PutUint64(dst[8:], entry.exp)
}

func (entry *rateLimiterItem) decode(src []byte) {
	if len(src) == rateLimiterItemPackedSizeInBytes {
		entry.currHits = binary.LittleEndian.Uint32(src)
		entry.prevHits = binary.LittleEndian.Uint32(src[4:])
		entry.exp = binary.LittleEndian.Uint64(src[8:])
	} else {
		entry.currHits = 0
		entry.prevHits = 0
		entry.exp = 0
	}
}

func (entry *rateLimiterItem) load() error {
	if entry.storage != nil {
		e, err := entry.storage.Get(entry.key)
		if err != nil {
			return err
		}
		entry.decode(e)
	} else {
		entry.decode(entry.cache.Get(nil, entry.key))
	}
	return nil
}

func (entry *rateLimiterItem) save(exp uint64) error {
	var buf [rateLimiterItemPackedSizeInBytes]byte

	entry.encode(buf[:])
	if entry.storage != nil {
		err := entry.storage.Set(entry.key, buf[:], time.Duration(exp)*time.Second)
		if err != nil {
			return err
		}
	} else {
		entry.cache.Set(entry.key, buf[:])
	}
	return nil
}

func (entry *rateLimiterItem) setExpiration(ts uint64, expiration uint64) {
	// Set the expiration if the entry does not exist.
	if entry.exp == 0 {
		entry.exp = ts + expiration
	} else if ts >= entry.exp {
		// The entry has expired, so handle the expiration.
		// Set prevHits to the current hits and reset currHits to 0.
		entry.prevHits = entry.currHits

		// Reset the current hits to 0.
		entry.currHits = 0

		// Check how far into the current window it is and set the
		// expiration accordingly; otherwise, it would reset only on
		// the next request and would not show the correct expiration.
		elapsed := ts - entry.exp
		if elapsed >= expiration {
			entry.exp = ts + expiration
		} else {
			entry.exp = ts + expiration - elapsed
		}
	}
}
