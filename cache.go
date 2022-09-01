package gw_cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// State that a cache entry could have
const (
	AVAILABLE = iota
	COMPUTING
	COMPUTED
	FAILED5xx
	FAILED4xx
)

const (
	Status4xx = iota
	Status4xxCached
	Status5xx
	Status5xxCached
)

// CacheEntry Every cache entry has this information
type CacheEntry struct {
	cacheKey              string     // key stringficated; needed for removal operation
	lock                  sync.Mutex // lock for repeated requests
	cond                  *sync.Cond // used in conjunction with the lock for repeated request until result is ready
	postProcessedResponse interface{}
	timestamp             time.Time // Last time accessed
	expirationTime        time.Time
	prev                  *CacheEntry
	next                  *CacheEntry
	state                 int8 // AVAILABLE, COMPUTING, etc
}

type RequestError struct {
	Error error
	Code  int
}

type CacheDriver struct {
	table             map[string]*CacheEntry
	missCount         int
	hitCount          int
	ttl               time.Duration
	head              CacheEntry // sentinel header node
	lock              sync.Mutex
	capacity          int
	extendedCapacity  int
	numEntries        int
	toCompress        bool
	toMapKey          func(key interface{}) (string, error)
	valueToBytes      func(value interface{}) ([]byte, error)
	bytesToValue      func([]byte) (interface{}, error)
	preProcessRequest func(request interface{}, other ...interface{}) (interface{}, *RequestError)
	callUServices     func(request, payload interface{}, other ...interface{}) (interface{}, *RequestError)
}

func (cache *CacheDriver) MissCount() int {
	return cache.missCount
}

func (cache *CacheDriver) HitCount() int {
	return cache.hitCount
}

func (cache *CacheDriver) Ttl() time.Duration {
	return cache.ttl
}

func (cache *CacheDriver) Capacity() int {
	return cache.capacity
}

func (cache *CacheDriver) ExtendedCapacity() int {
	return cache.extendedCapacity
}

func (cache *CacheDriver) NumEntries() int {
	return cache.numEntries
}

// New Creates a new cache. Parameters are:
//
// capacity: maximum number of entries that cache can manage without evicting the least recently used
//
// capFactor is a number in (0.1, 3] that indicates how long the cache should be oversize in order to avoid rehashing
//
// ttl: time to live of a cache entry
//
// toMapKey is a function in charge of transforming the request into a string
//
// preProcessRequest is an optional function that could be used for validation, transforming
// the request in a more suitable form, etc.
//
// callUService: is responsible for calling to the service and building a byte sequence corresponding to the
// service response
//
func New(capacity int, capFactor float64, ttl time.Duration,
	toMapKey func(key interface{}) (string, error),
	preProcessRequest func(request interface{}, other ...interface{}) (interface{}, *RequestError),
	callUServices func(request, payload interface{}, other ...interface{}) (interface{}, *RequestError),
) *CacheDriver {

	if capFactor < 0.1 || capFactor > 3.0 {
		panic(fmt.Sprintf("invalid capFactor %f. It should be in [0.1, 3]",
			capFactor))
	}

	extendedCapacity := math.Ceil((1.0 + capFactor) * float64(capacity))
	ret := &CacheDriver{
		missCount:         0,
		hitCount:          0,
		capacity:          capacity,
		extendedCapacity:  int(extendedCapacity),
		numEntries:        0,
		ttl:               ttl,
		table:             make(map[string]*CacheEntry, int(extendedCapacity)),
		toMapKey:          toMapKey,
		preProcessRequest: preProcessRequest,
		callUServices:     callUServices,
	}
	ret.head.prev = &ret.head
	ret.head.next = &ret.head

	return ret
}

// NewWithCompression Creates a new cache with compressed entries.
//
// The constructor is some similar to the version that does not compress. The difference is
// that in order to compress, the cache needs a serialized representation of what will be
// stored into the cache. For that reason, the constructor receives two additional functions.
// The first function, ValueToBytes transforms the value into a byte slice (type []byte). The
// second function, bytesToValue, takes a serialized representation of the value stored into the
// cache, and it transforms it to the original representation.
//
// Parameters are:
//
// capacity: maximum number of entries that cache can manage without evicting the least recently used
//
// capFactor is a number in (0.1, 3] that indicates how long the cache should be oversize in order to avoid rehashing
//
// ttl: time to live of a cache entry
//
// toMapKey is a function in charge of transforming the request into a string
//
// valueToBytes transforms the value into a []byte
//
// bytesToValue transforms a []byte into the original value representation
//
// valueToBytes
//
// preProcessRequest is an optional function that could be used for validation, transforming
// the request in a more suitable form, etc.
//
// callUService: is responsible for calling to the service and building a byte sequence corresponding to the
// service response
//
func NewWithCompression(capacity int, capFactor float64, ttl time.Duration,
	toMapKey func(key interface{}) (string, error),
	valueToBytes func(value interface{}) ([]byte, error),
	bytesToValue func([]byte) (interface{}, error),
	preProcessRequest func(request interface{}, other ...interface{}) (interface{}, *RequestError),
	callUServices func(request, payload interface{},
		other ...interface{}) (interface{}, *RequestError),
) (cache *CacheDriver) {

	cache = New(capacity, capFactor, ttl, toMapKey, preProcessRequest, callUServices)
	if cache != nil {
		cache.toCompress = true
		cache.valueToBytes = valueToBytes
		cache.bytesToValue = bytesToValue
	}

	return cache
}

// Insert entry as the first item of cache (mru)
func (cache *CacheDriver) insertAsMru(entry *CacheEntry) {
	entry.prev = &cache.head
	entry.next = cache.head.next
	cache.head.next.prev = entry
	cache.head.next = entry
}

// Auto deletion of lru queue
func (entry *CacheEntry) selfDeleteFromLRUList() {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
}

func (cache *CacheDriver) becomeMru(entry *CacheEntry) {
	entry.selfDeleteFromLRUList()
	cache.insertAsMru(entry)
}

// Rewove the last item in the list (lru); mutex must be taken. The entry becomes AVAILABLE
func (cache *CacheDriver) evictLruEntry() (*CacheEntry, error) {
	entry := cache.head.prev // <-- LRU entry
	if entry.state == COMPUTING {
		err := errors.New("LRU entry is in COMPUTING state. This could be a bug or a cache misconfiguration")
		return nil, err
	}
	entry.selfDeleteFromLRUList()
	delete(cache.table, entry.cacheKey) // Key evicted
	return entry, nil
}

func (cache *CacheDriver) allocateEntry(cacheKey string,
	currTime time.Time) (entry *CacheEntry, err error) {

	if cache.numEntries == cache.capacity {
		entry, err = cache.evictLruEntry()
		if err != nil {
			return nil, err
		}
	} else {
		entry = new(CacheEntry)
		entry.cond = sync.NewCond(&entry.lock)
		cache.numEntries++
	}
	cache.insertAsMru(entry)
	entry.cacheKey = cacheKey
	entry.state = AVAILABLE
	entry.timestamp = currTime
	entry.expirationTime = currTime.Add(cache.ttl)
	entry.postProcessedResponse = nil // should dispose any allocated result
	cache.table[cacheKey] = entry
	return entry, nil
}

// RetrieveFromCacheOrCompute Search Request in the cache. If the request is already computed, then it
// immediately returns the cached entry. If the request is the first, then it blocks until the result is
// ready. If the request is not the first but the result is not still ready, then it blocks
// until the result is ready
func (cache *CacheDriver) RetrieveFromCacheOrCompute(request interface{},
	other ...interface{}) (interface{}, *RequestError) {

	var requestError *RequestError

	var payload interface{}
	if cache.preProcessRequest != nil {
		payload, requestError = cache.preProcessRequest(request, other...)
		if requestError != nil {
			return nil, requestError
		}
	} else {
		payload = request
	}

	cacheKey, err := cache.toMapKey(payload)
	if err != nil {
		return nil, &RequestError{
			Error: err,
			Code:  Status4xx,
		}
	}

	var entry *CacheEntry
	var hit bool
	currTime := time.Now()
	cache.lock.Lock()

	withCompression := cache.toCompress

	entry, hit = cache.table[cacheKey]
	if hit && currTime.Before(entry.expirationTime) {
		cache.hitCount++
		cache.becomeMru(entry)
		cache.lock.Unlock()

		entry.lock.Lock()              // will block if it is computing
		for entry.state == COMPUTING { // this guard is for protection; it should never be true
			entry.cond.Wait() // it will wake up when result arrives
		}
		defer entry.lock.Unlock()
		if entry.state == FAILED5xx {
			return nil, &RequestError{
				Error: errors.New("uservice failed to preProcessRequest the match state (cached)"),
				Code:  Status5xxCached, // include 4xx and 5xx
			}
		} else if entry.state == FAILED4xx {
			return nil, &RequestError{
				Error: errors.New("uservice failed to preProcessRequest the match state (cached)"),
				Code:  Status4xxCached, // include 4xx and 5xx
			}
		}
		entry.timestamp = currTime
		entry.expirationTime = currTime.Add(cache.ttl)

		if withCompression {
			buf, err := lz4Decompress(entry.postProcessedResponse.([]byte))
			if err != nil {
				return nil, &RequestError{
					Error: errors.New("cannot decompress stored message"),
					Code:  Status5xx, // include 4xx and 5xx
				}
			}
			result, err := cache.bytesToValue(buf)
			if err != nil {
				return nil, &RequestError{
					Error: errors.New("cannot convert decompressed stored message"),
					Code:  Status5xx, // include 4xx and 5xx
				}
			}
			return result, nil
		}

		return entry.postProcessedResponse, nil
	}

	// In this point global cache lock is taken
	// Request is not in cache
	entry, err = cache.allocateEntry(cacheKey, currTime)
	if err != nil {
		cache.lock.Unlock() // an error getting cache entry ==> we invoke directly the uservice
		return cache.callUServices(request, payload, other...)
	}

	entry.state = COMPUTING

	cache.missCount++
	cache.lock.Unlock() // release global lock before to take the entry lock

	entry.lock.Lock() // other requests will wait for until postProcessedResponse is gotten

	var retVal interface{} = nil // Explicit initialization for understanding flow!
	retVal, requestError = cache.callUServices(request, payload, other...)
	if requestError != nil {
		switch {
		case requestError.Code == Status4xx || requestError.Code == Status4xxCached:
			entry.state = FAILED4xx
		case requestError.Code == Status5xx || requestError.Code == Status5xxCached:
			entry.state = FAILED5xx
		}
	} else {
		entry.state = COMPUTED
	}

	if withCompression {
		buf, err := cache.valueToBytes(retVal) // transforms retVal into a []byte ready for compression
		if err != nil {
			entry.state = FAILED5xx
		}
		lz4Buf, err := lz4Compress(buf)
		if err != nil {
			entry.state = FAILED5xx
		}
		entry.postProcessedResponse = lz4Buf // stores the compressed representation
	} else {
		entry.postProcessedResponse = retVal
	}

	entry.lock.Unlock()
	entry.cond.Broadcast() // wake up eventual requests waiting for the result (which has failed!)

	return retVal, requestError
}

// remove entry from cache.Mutex must be taken
func (cache *CacheDriver) remove(entry *CacheEntry) {
	entry.selfDeleteFromLRUList()
	delete(cache.table, entry.cacheKey)
	cache.numEntries--
}

// has return true is state in the cache
func (cache *CacheDriver) has(val interface{}) bool {
	key, err := cache.toMapKey(val)
	if err != nil {
		return false
	}
	entry, hit := cache.table[key]
	return hit && time.Now().Before(entry.expirationTime)
}

// Return the lru without moving it from the queue
func (cache *CacheDriver) getLru() *CacheEntry {
	if cache.numEntries == 0 {
		return nil
	}
	return cache.head.prev
}

// Return the mru without moving it from the queue
func (cache *CacheDriver) getMru() *CacheEntry {
	if cache.numEntries == 0 {
		return nil
	}
	return cache.head.next
}

// CacheIt Iterator on cache entries. Go from MUR to LRU
type CacheIt struct {
	cachePtr *CacheDriver
	curr     *CacheEntry
}

func (cache *CacheDriver) NewCacheIt() *CacheIt {
	return &CacheIt{cachePtr: cache, curr: cache.head.next}
}

func (it *CacheIt) HasCurr() bool {
	return it.curr != &it.cachePtr.head
}

func (it *CacheIt) GetCurr() *CacheEntry {
	return it.curr
}

func (it *CacheIt) Next() *CacheEntry {
	if !it.HasCurr() {
		return nil
	}
	it.curr = it.curr.next
	return it.curr
}

type CacheState struct {
	MissCount  int
	HitCount   int
	TTL        time.Duration
	Capacity   int
	NumEntries int
}

// GetState Return a json containing the cache state. Use the internal mutex. Be careful with a deadlock
func (cache *CacheDriver) GetState() (string, error) {

	cache.lock.Lock()
	defer cache.lock.Unlock()

	state := CacheState{
		MissCount:  cache.missCount,
		HitCount:   cache.hitCount,
		TTL:        cache.ttl,
		Capacity:   cache.capacity,
		NumEntries: cache.numEntries,
	}

	buf, err := json.MarshalIndent(&state, "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// helper that does not take lock
func (cache *CacheDriver) clean() error {

	for it := cache.NewCacheIt(); it.HasCurr(); it.Next() {
		entry := it.GetCurr()
		if entry.state == COMPUTING {
			return errors.New("cannot clean cache because a entry was found waiting for response")
		}
	}

	// Now that we know that we can clean safely, we pass again and mark all the entries as AVAILABLE
	for it := cache.NewCacheIt(); it.HasCurr(); it.Next() {
		it.GetCurr().state = AVAILABLE
	}

	// At this point all the entries are marked as AVAILABLE ==> we reset
	cache.numEntries = 0
	cache.hitCount = 0
	cache.missCount = 0

	return nil
}

// Clean Try to clean the cache. All the entries are deleted and counters reset. Fails if any entry is in COMPUTING
// state.
//
// Uses internal lock
//
func (cache *CacheDriver) Clean() error {

	cache.lock.Lock()
	defer cache.lock.Unlock()

	return cache.clean()
}

func (cache *CacheDriver) Set(capacity int, ttl time.Duration) error {

	cache.lock.Lock()
	defer cache.lock.Unlock()

	if capacity != 0 {
		if cache.numEntries > capacity {
			return errors.New("number of entries in the cache is greater than given capacity")
		}
		cache.capacity = capacity
	}

	if ttl != 0 {
		for it := cache.NewCacheIt(); it.HasCurr(); it.Next() {
			entry := it.GetCurr()
			if entry.state != COMPUTED {
				continue
			}
			entry.expirationTime = entry.timestamp.Add(ttl)
		}
		cache.ttl = ttl
	}

	return nil
}
