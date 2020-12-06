package gw_cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	AVAILABLE = iota
	COMPUTING
	COMPUTED
	FAILED
)

const (
	Status4xx = iota
	Status5xx
	Status5xxCached
)

type CacheEntry struct {
	cacheKey              string // key stringficated; needed for removal operation
	lock                  sync.Mutex
	cond                  *sync.Cond
	postProcessedResponse interface{}
	timestamp             time.Time // Last time accessed
	expirationTime        time.Time
	prev                  *CacheEntry
	next                  *CacheEntry
	state                 int8
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

// Creates a new cache. Parameters are:
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

	ret := &CacheDriver{
		missCount:         0,
		hitCount:          0,
		capacity:          capacity,
		numEntries:        0,
		ttl:               ttl,
		table:             make(map[string]*CacheEntry),
		toMapKey:          toMapKey,
		preProcessRequest: preProcessRequest,
		callUServices:     callUServices,
	}
	ret.head.prev = &ret.head
	ret.head.next = &ret.head

	return ret
}

// Insert entry as the first item of cache (mru)
func (cache *CacheDriver) InsertAsMru(entry *CacheEntry) {
	entry.prev = &cache.head
	entry.next = cache.head.next
	cache.head.next.prev = entry
	cache.head.next = entry
}

// Auto deletion of lru queue
func (entry *CacheEntry) SelfDeleteFromLRUList() {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
}

func (cache *CacheDriver) BecomeMru(entry *CacheEntry) {
	entry.SelfDeleteFromLRUList()
	cache.InsertAsMru(entry)
}

// Rewove the last item in the list (lru); mutex must be taken. The entry becomes AVAILABLE
func (cache *CacheDriver) EvictLruEntry() (*CacheEntry, error) {
	entry := cache.head.prev // <-- LRU entry
	if entry.state == COMPUTING {
		err := errors.New("LRU entry is in COMPUTING state. This could be a bug or a cache misconfiguration")
		return nil, err
	}
	entry.SelfDeleteFromLRUList()
	delete(cache.table, entry.cacheKey) // Key evicted
	return entry, nil
}

func (cache *CacheDriver) AllocateEntry(cacheKey string,
	currTime time.Time) (entry *CacheEntry, err error) {

	if cache.numEntries == cache.capacity {
		entry, err = cache.EvictLruEntry()
		if err != nil {
			return nil, err
		}
	} else {
		entry = new(CacheEntry)
		entry.cond = sync.NewCond(&entry.lock)
		cache.numEntries++
	}
	cache.InsertAsMru(entry)
	entry.cacheKey = cacheKey
	entry.state = AVAILABLE
	entry.timestamp = currTime
	entry.expirationTime = currTime.Add(cache.ttl)
	entry.postProcessedResponse = nil // should dispose any allocated result
	cache.table[cacheKey] = entry
	return entry, nil
}

// Search Request in the cache and return a locked entry
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
		fmt.Println("HIT")
		cache.BecomeMru(entry)
		cache.lock.Unlock()

		entry.lock.Lock()              // will block if it is computing
		for entry.state == COMPUTING { // this guard is for protection; it should never be true
			fmt.Println("Waiting for uservices completion")
			entry.cond.Wait() // it will wake up when result arrives
			fmt.Println("Woke up!")
		}
		defer entry.lock.Unlock()
		if entry.state == FAILED {
			return nil, &RequestError{
				Error: errors.New("uservice failed to preProcessRequest the match state (cached)"),
				Code:  Status5xxCached,
			}
		}
		entry.timestamp = currTime
		entry.expirationTime = currTime.Add(cache.ttl)
		return entry.postProcessedResponse, nil
	}

	// In this point global cache lock is taken
	// Request is not in cache
	entry, err = cache.AllocateEntry(cacheKey, currTime)
	if err != nil {
		cache.lock.Unlock() // an error getting cache entry ==> we invoke directly the uservice
		return cache.callUServices(request, payload, other...)
	}

	entry.state = COMPUTING

	fmt.Println("MISS")
	cache.missCount++
	cache.lock.Unlock() // release global lock before to take the entry lock

	entry.lock.Lock() // other requests will wait for until postProcessedResponse is gotten

	var retVal interface{} = nil // Explicit initialization for understanding flow!
	retVal, requestError = cache.callUServices(request, payload, other...)
	if requestError != nil {
		entry.state = FAILED
	} else {
		entry.state = COMPUTED
	}
	entry.postProcessedResponse = retVal
	entry.lock.Unlock()
	entry.cond.Broadcast() // wake up eventual requests waiting for the result (which has failed!)

	return retVal, requestError
}

// Remove entry from cache.Mutex must be taken
func (cache *CacheDriver) Remove(entry *CacheEntry) {
	entry.SelfDeleteFromLRUList()
	delete(cache.table, entry.cacheKey)
	cache.numEntries--
}

// Has return true is state in the cache
func (cache *CacheDriver) Has(val interface{}) bool {
	key, err := cache.toMapKey(val)
	if err != nil {
		return false
	}
	entry, hit := cache.table[key]
	return hit && time.Now().Before(entry.expirationTime)
}

// Return the lru without moving it from the queue
func (cache *CacheDriver) GetLru() *CacheEntry {
	if cache.numEntries == 0 {
		return nil
	}
	return cache.head.prev
}

// Return the mru without moving it from the queue
func (cache *CacheDriver) GetMru() *CacheEntry {
	if cache.numEntries == 0 {
		return nil
	}
	return cache.head.next
}

// Iterator on cache entries. Go from MUR to LRU
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

// Return a json containing the cache state. Use the internal mutex. Be careful with a deadlock
func (cache *CacheDriver) GetState() ([]byte, error) {

	cache.lock.Lock()
	defer cache.lock.Unlock()

	state := CacheState{
		MissCount:  cache.missCount,
		HitCount:   cache.hitCount,
		TTL:        cache.ttl,
		Capacity:   cache.capacity,
		NumEntries: cache.numEntries,
	}

	return json.Marshal(&state)
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

// Try to clean the cache. All the entries are deleted and counters reset. Fails if any entry is in COMPUTING
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
