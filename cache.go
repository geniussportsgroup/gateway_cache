package gw_cache

import (
	"encoding/json"
	"errors"
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

// CacheCapacityFactor factor by which capacity is increased so to mitigate table resizes
const CacheCapacityFactor = 0.30

// CacheEntry Every cache entry has this information
type CacheEntry struct {
	cacheKey              string     // key stringficated; needed for removal operation
	lock                  sync.Mutex // lock for repeated requests
	cond                  *sync.Cond // used in conjunction with the lock for repeating repeated request until result is ready
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
	numEntries        int
	toMapKey          func(key interface{}) (string, error)
	preProcessRequest func(request interface{}, other ...interface{}) (interface{}, *RequestError)
	callUServices     func(request, payload interface{}, other ...interface{}) (interface{}, *RequestError)
}

// New Creates a new cache. Parameters are:
//
// capacity: maximum number of entries that cache can manage without evicting the least recentrly used
// ttl: time to live of a cache entry
//
// callUService: is responsible of calling to the service and building a byte sequence corresponding to the
// service response
//
func New(capacity int, ttl time.Duration,
	toMapKey func(key interface{}) (string, error),
	preProcessRequest func(request interface{}, other ...interface{}) (interface{}, *RequestError),
	callUServices func(request, payload interface{}, other ...interface{}) (interface{}, *RequestError),
) *CacheDriver {

	extendedCapacity := math.Ceil(1.0 * CacheCapacityFactor * float64(capacity))
	ret := &CacheDriver{
		missCount:         0,
		hitCount:          0,
		capacity:          capacity,
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
// immediately return the cached entry. If the request is the first, then it blocks until the result is
// ready. If the request is not the first but the result is not still ready, the it blocks until the result is ready
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
	entry.postProcessedResponse = retVal
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
