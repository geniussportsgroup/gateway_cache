package gw_cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/geniussportsgroup/gateway_cache/v3/reporter"
)

// State that a cache entry could have
const (
	AVAILABLE EntryState = iota
	COMPUTING
	COMPUTED
	FAILED5xx
	FAILED4xx
	FAILED5XXMISSHANDLERERROR
)

const (
	Status4xx CodeStatus = iota
	Status4xxCached
	Status5xx
	Status5xxCached
	StatusUser
)

// CacheEntry Every cache entry has this information
type CacheEntry[K any] struct {
	cacheKey                        string     // key stringficated; needed for removal operation
	lock                            sync.Mutex // lock for repeated requests
	cond                            *sync.Cond // used in conjunction with the lock for repeated request until result is ready
	postProcessedResponse           K
	postProcessedResponseCompressed []byte
	timestamp                       time.Time // Last time accessed
	expirationTime                  time.Time
	prev                            *CacheEntry[K]
	next                            *CacheEntry[K]
	state                           EntryState // AVAILABLE, COMPUTING, etc
	err                             error
}

// CacheDriver The cache itself.
//
// K represents the request's type this will be used as key.
//
// T the response's type this will be used as value.
type CacheDriver[K any, T any] struct {
	table            map[string]*CacheEntry[T]
	missCount        int
	hitCount         int
	ttl              time.Duration
	ttlForNegative   time.Duration
	head             CacheEntry[T] // sentinel header node
	lock             sync.Mutex
	capacity         int
	extendedCapacity int
	numEntries       int
	toCompress       bool
	// processor        ProcessorI[
	cacheMissSolver func(K, ...interface{}) (T, *RequestError) //we will leave the pre process logic for this function
	toMapKey        func(K) (string, error)
	transformer     TransformerI[T]
	compressor      CompressorI
	reporter        Reporter
}

func (cache *CacheDriver[T, K]) MissCount() int {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	return cache.missCount
}

func (cache *CacheDriver[T, K]) HitCount() int {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	return cache.hitCount
}

func (cache *CacheDriver[T, K]) Ttl() time.Duration {
	return cache.ttl
}

func (cache *CacheDriver[T, K]) Capacity() int {
	return cache.capacity
}

func (cache *CacheDriver[T, K]) ExtendedCapacity() int {
	return cache.extendedCapacity
}

func (cache *CacheDriver[T, K]) NumEntries() int {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	return cache.numEntries
}

func (cache *CacheDriver[T, K]) TTLForNegative() time.Duration {
	return cache.ttlForNegative
}

// LazyRemove removes the entry with keyVal from the cache. It does not remove the entry immediately, but it marks it as	removed.
func (cache *CacheDriver[T, K]) LazyRemove(keyVal T) error {

	key, err := cache.toMapKey(keyVal)
	if err != nil {
		return err
	}

	cache.lock.Lock()

	if entry, ok := cache.table[key]; ok {
		cache.lock.Unlock()
		entry.lock.Lock()
		defer entry.lock.Unlock()

		if entry.expirationTime.Before(time.Now()) {
			return ErrEntryExpired
		}

		if entry.state != COMPUTING && entry.state != AVAILABLE {
			// In this way, when the entry is accessed again, it will be removed
			entry.timestamp = time.Now()
			entry.expirationTime = entry.timestamp
			cache.lock.Lock()
			cache.becomeLru(entry)
			cache.lock.Unlock()
			return nil
		}

		if entry.state == AVAILABLE {
			return ErrEntryAvailableState
		}

		return ErrEntryComputingState
	}

	cache.lock.Unlock()

	return nil
}

func (cache *CacheDriver[T, K]) Touch(keyVal T) error {

	key, err := cache.toMapKey(keyVal)
	if err != nil {
		return err
	}

	cache.lock.Lock()

	if entry, ok := cache.table[key]; ok {
		cache.lock.Unlock()
		entry.lock.Lock()
		defer entry.lock.Unlock()

		currentTime := time.Now()
		if entry.expirationTime.Before(currentTime) {
			return ErrEntryExpired
		}

		if entry.state != COMPUTING && entry.state != AVAILABLE {
			// In this way, when the entry is accessed again, it will be removed
			entry.timestamp = currentTime
			entry.expirationTime = currentTime.Add(cache.ttl)
			cache.lock.Lock()
			cache.becomeMru(entry)
			cache.lock.Unlock()
			return nil
		}

		if entry.state == AVAILABLE {
			return ErrEntryAvailableState
		}

		return ErrEntryComputingState
	}

	cache.lock.Unlock()

	return nil
}

// New Creates a new cache. Parameters are:
//
// capacity: maximum number of entries that cache can manage without evicting the least recently used
//
// capFactor is a number in (0.1, 3] that indicates how long the cache should be oversize in order to avoid rehashing
//
// ttl: time to live of a cache entry
//
// processor: is an interface that must be implemented by the user. It is in charge of transforming the request
// into a string and get the value in case that does not exist in the cache
//
//	type ProcessorI[K any, T any] interface {
//		ToMapKey(keyVal T) (string, error) //Is the function in charge of transforming the request into a string
//		CacheMissSolver(K) (T, *models.RequestError)  //Is the function in charge of getting the value in case that does not exist in the cache
//	}

type Options[K, T any] func(*CacheDriver[K, T])

func New[K any, T any](
	capacity int,
	capFactor float64,
	ttl time.Duration,
	ttlForNegative time.Duration,
	missSolver func(K, ...interface{}) (T, *RequestError),
	toMapKey func(K) (string, error),
	options ...Options[K, T],
) *CacheDriver[K, T] {

	if capFactor < 0.1 || capFactor > 3.0 {
		panic(fmt.Sprintf("invalid capFactor %f. It should be in [0.1, 3]",
			capFactor))
	}

	extendedCapacity := math.Ceil((1.0 + capFactor) * float64(capacity))
	ret := &CacheDriver[K, T]{
		missCount:        0,
		hitCount:         0,
		capacity:         capacity,
		extendedCapacity: int(extendedCapacity),
		numEntries:       0,
		ttl:              ttl,
		ttlForNegative:   ttlForNegative,
		table:            make(map[string]*CacheEntry[T], int(extendedCapacity)),
		cacheMissSolver:  missSolver,
		toMapKey:         toMapKey,
		compressor:       lz4Compressor{},
		reporter:         &reporter.Default{},
	}
	ret.head.prev = &ret.head
	ret.head.next = &ret.head

	for _, option := range options {
		option(ret)
	}

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
// The parameters are:
//
// capacity: maximum number of entries that cache can manage without evicting the least recently used
//
// capFactor is a number in (0.1, 3] that indicates how long the cache should be oversize in order to avoid rehashing
//
// ttl: time to live of a cache entry
//
// processor: is an interface that must be implemented by the user. It is in charge of transforming the request
// into a string and get the value in case that does not exist in the cache
//
//	type ProcessorI[K any, T any] interface {
//		ToMapKey(keyVal T) (string, error) //Is the function in charge of transforming the request into a string
//		CacheMissSolver(K) (T, *models.RequestError)  //Is the function in charge of getting the value in case that does not exist in the cache
//	}
//
// transformer: is an interface that must be implemented by the user. It is in charge of transforming the value
// into a byte slice and vice versa
//
//	type Transformer[T any] interface {
//		ValueToBytes(T) ([]byte, error)
//		BytesToValue([]byte) (T, error)
//	}
//
// it is a default implementation of the transformer interface that you can use
//
//	type DefaultTransformer[T any] struct{}
//
//	func (_ *DefaultTransformer[T]) BytesToValue(in []byte) (T, error) {
//		var out T
//		err := json.Unmarshal(in, &out)
//		if err != nil {
//			return out, err
//		}
//		return out, nil
//	}
//
//	func (_ *DefaultTransformer[T]) ValueToBytes(in T) ([]byte, error) {
//			return json.Marshal(in)
//		}
func NewWithCompression[K any, T any](
	capacity int,
	capFactor float64,
	ttl time.Duration,
	ttlForNegative time.Duration,
	missSolver func(K, ...interface{}) (T, *RequestError),
	toMapKey func(K) (string, error),
	compressor TransformerI[T],
	options ...Options[K, T],
) (cache *CacheDriver[K, T]) {

	cache = New(capacity, capFactor, ttl, ttlForNegative, missSolver, toMapKey, options...)
	if cache != nil {
		cache.toCompress = true
		cache.transformer = compressor
	}

	return cache
}

func (cache *CacheDriver[T, K]) SetReporter(reporter Reporter) {
	cache.reporter = reporter
}

// Insert entry as the first item of cache (mru)
func (cache *CacheDriver[T, K]) insertAsMru(entry *CacheEntry[K]) {
	entry.prev = &cache.head
	entry.next = cache.head.next
	cache.head.next.prev = entry
	cache.head.next = entry
}

func (cache *CacheDriver[T, K]) insertAsLru(entry *CacheEntry[K]) {
	entry.prev = cache.head.prev
	entry.next = &cache.head
	cache.head.prev.next = entry
	cache.head.prev = entry
}

// Auto deletion of lru queue
func (entry *CacheEntry[T]) selfDeleteFromLRUList() {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
}

func (cache *CacheDriver[T, K]) isLru(entry *CacheEntry[K]) bool {
	return entry.next == &cache.head
}

func (cache *CacheDriver[T, K]) isMru(entry *CacheEntry[K]) bool {
	return entry.prev == &cache.head
}

func (cache *CacheDriver[T, K]) isKeyLru(keyVal T) (bool, error) {
	key, err := cache.toMapKey(keyVal)
	if err != nil {
		return false, err
	}

	if entry, ok := cache.table[key]; ok {
		return cache.isLru(entry), nil
	}

	return false, nil
}

func (cache *CacheDriver[T, K]) isKeyMru(keyVal T) (bool, error) {

	key, err := cache.toMapKey(keyVal)
	if err != nil {
		return false, err
	}

	if entry, ok := cache.table[key]; ok && entry.expirationTime.After(time.Now()) {
		return cache.isMru(entry), nil
	}

	return false, nil
}

// func (cache *CacheDriver[T, K]) becomeMru(entry *CacheEntry[K]) {
func (cache *CacheDriver[T, K]) becomeMru(entry *CacheEntry[K]) {
	entry.selfDeleteFromLRUList()
	cache.insertAsMru(entry)
}

func (cache *CacheDriver[T, K]) becomeLru(entry *CacheEntry[K]) {
	entry.selfDeleteFromLRUList()
	cache.insertAsLru(entry)
}

// Rewove the last item in the list (lru); mutex must be taken. The entry becomes AVAILABLE
func (cache *CacheDriver[T, K]) evictLruEntry() (*CacheEntry[K], error) {
	entry := cache.head.prev // <-- LRU entry
	if entry.state == COMPUTING {
		return nil, ErrLRUComputing
	}
	entry.selfDeleteFromLRUList()
	cache.table[entry.cacheKey] = nil
	delete(cache.table, entry.cacheKey) // Key evicted
	return entry, nil
}

func (cache *CacheDriver[T, K]) allocateEntry(
	cacheKey string,
	currTime time.Time) (entry *CacheEntry[K], err error) {

	if cache.numEntries == cache.capacity {
		entry, err = cache.evictLruEntry()
		if err != nil {
			return nil, err
		}
	} else {
		entry = new(CacheEntry[K])
		entry.cond = sync.NewCond(&entry.lock)
		cache.numEntries++
	}
	cache.insertAsMru(entry)
	entry.cacheKey = cacheKey
	entry.state = AVAILABLE
	entry.timestamp = currTime
	entry.expirationTime = currTime.Add(cache.ttl)
	var zeroK K
	entry.postProcessedResponse = zeroK // should dispose any allocated result
	cache.table[cacheKey] = entry
	return entry, nil
}

// RetrieveFromCacheOrCompute Search Request in the cache. If the request is already computed, then it
// immediately returns the cached entry. If the request is the first, then it blocks until the result is
// ready. If the request is not the first but the result is not still ready, then it blocks
// until the result is ready
func (cache *CacheDriver[T, K]) RetrieveFromCacheOrCompute(request T,
	other ...interface{}) (K, *RequestError) {

	var requestError *RequestError
	var zeroK K
	payload := request

	cacheKey, err := cache.toMapKey(payload)
	if err != nil {
		return zeroK, &RequestError{
			Error: err,
			Code:  Status4xx,
		}
	}

	var entry *CacheEntry[K]
	var hit bool
	currTime := time.Now()
	cache.lock.Lock()

	withCompression := cache.toCompress

	entry, hit = cache.table[cacheKey]
	if hit && currTime.Before(entry.expirationTime) {
		cache.hitCount++
		go cache.reporter.ReportHit()
		cache.becomeMru(entry) //TODO: check if it is negative
		cache.lock.Unlock()
		entry.lock.Lock()              // will block if it is computing
		for entry.state == COMPUTING { // this guard is for protection; it should never be true
			entry.cond.Wait() // it will wake up when result arrives
		}
		defer entry.lock.Unlock()

		if entry.state == FAILED5xx {
			// entry.expirationTime = currTime.Add(cache.ttlForNegative)
			return zeroK, &RequestError{
				Error: entry.err,
				Code:  Status5xxCached, // include 4xx and 5xx
			}
		} else if entry.state == FAILED4xx {
			// entry.expirationTime = currTime.Add(cache.ttlForNegative)
			return zeroK, &RequestError{
				Error: entry.err,
				Code:  Status4xxCached, // include 4xx and 5xx
			}
		}

		entry.timestamp = currTime
		entry.expirationTime = currTime.Add(cache.ttl)

		if withCompression {

			buf, err := cache.compressor.Decompress(entry.postProcessedResponseCompressed)
			if err != nil {
				return zeroK, &RequestError{
					Error: errors.New("cannot decompress stored message"),
					Code:  Status5xx, // include 4xx and 5xx
				}
			}

			result, err := cache.transformer.BytesToValue(buf)
			if err != nil {
				return zeroK, &RequestError{
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
		// return cache.callUServices(request, payload, other...)
		return cache.cacheMissSolver(request, other...)
	}

	entry.state = COMPUTING

	go cache.reporter.ReportMiss()
	cache.missCount++
	cache.lock.Unlock() // release global lock before to take the entry lock
	entry.lock.Lock()   // other requests will wait for until postProcessedResponse is gotten
	defer entry.lock.Unlock()

	retVal, requestError := cache.cacheMissSolver(request, other...)
	if requestError != nil {
		switch requestError.Code {
		case Status4xx, Status4xxCached:
			entry.state = FAILED4xx
		case Status5xx, Status5xxCached:
			entry.state = FAILED5xx
		default:
			entry.state = FAILED5XXMISSHANDLERERROR
		}
		entry.err = requestError.Error
		entry.expirationTime = currTime.Add(cache.ttlForNegative)
	} else {
		entry.state = COMPUTED
	}

	if withCompression {
		buf, err := cache.transformer.ValueToBytes(retVal) // transforms retVal into a []byte ready for compression
		if err != nil {
			entry.state = FAILED5xx
		}
		lz4Buf, err := cache.compressor.Compress(buf)
		if err != nil {
			entry.state = FAILED5xx
			entry.postProcessedResponse = retVal
			entry.expirationTime = currTime.Add(cache.ttlForNegative)
		} else {
			entry.postProcessedResponseCompressed = lz4Buf
		}
	}

	entry.postProcessedResponse = retVal
	entry.cond.Broadcast() // wake up eventual requests waiting for the result (which has failed!)

	return retVal, requestError
}

// remove entry from cache.Mutex must be taken
func (cache *CacheDriver[T, K]) remove(entry *CacheEntry[K]) {
	entry.selfDeleteFromLRUList()
	cache.table[entry.cacheKey] = nil
	delete(cache.table, entry.cacheKey)
	cache.numEntries--
}

// has return true is state in the cache
func (cache *CacheDriver[T, K]) has(val T) bool {
	key, err := cache.toMapKey(val)
	// key, err := cache.toMapKey(val)
	if err != nil {
		return false
	}
	entry, hit := cache.table[key]
	return hit && time.Now().Before(entry.expirationTime)
}

// Return the lru without moving it from the queue
func (cache *CacheDriver[T, K]) getLru() *CacheEntry[K] {
	if cache.numEntries == 0 {
		return nil
	}
	return cache.head.prev
}

// Return the mru without moving it from the queue
func (cache *CacheDriver[T, K]) getMru() *CacheEntry[K] {
	if cache.numEntries == 0 {
		return nil
	}
	return cache.head.next
}

// CacheIt Iterator on cache entries. Go from MUR to LRU
type CacheIt[T any, K any] struct {
	cachePtr *CacheDriver[T, K]
	curr     *CacheEntry[K]
}

func (cache *CacheDriver[T, K]) NewCacheIt() *CacheIt[T, K] {
	return &CacheIt[T, K]{cachePtr: cache, curr: cache.head.next}
}

func (it *CacheIt[T, K]) HasCurr() bool {
	return it.curr != &it.cachePtr.head
}

func (it *CacheIt[T, K]) GetCurr() *CacheEntry[K] {
	return it.curr
}

func (it *CacheIt[T, K]) Next() *CacheEntry[K] {
	if !it.HasCurr() {
		return nil
	}
	it.curr = it.curr.next
	return it.curr
}

type CacheState struct {
	MissCount      int
	HitCount       int
	TTL            time.Duration
	TTLForNegative time.Duration
	Capacity       int
	NumEntries     int
}

// GetState Return a json containing the cache state. Use the internal mutex. Be careful with a deadlock
func (cache *CacheDriver[T, K]) GetState() (string, error) {

	cache.lock.Lock()
	defer cache.lock.Unlock()

	state := CacheState{
		MissCount:      cache.missCount,
		HitCount:       cache.hitCount,
		TTL:            cache.ttl,
		TTLForNegative: cache.ttlForNegative,
		Capacity:       cache.capacity,
		NumEntries:     cache.numEntries,
	}

	buf, err := json.MarshalIndent(&state, "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// helper that does not take lock
func (cache *CacheDriver[T, K]) clean() error {

	for it := cache.NewCacheIt(); it.HasCurr(); it.Next() {
		entry := it.GetCurr()
		if entry.state == COMPUTING {
			return ErrEntryComputingState
			// return errors.New("cannot clean cache because a entry was found waiting for response")
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
func (cache *CacheDriver[T, K]) Clean() error {

	cache.lock.Lock()
	defer cache.lock.Unlock()

	return cache.clean()
}

func (cache *CacheDriver[T, K]) Set(capacity int, ttl time.Duration, ttlForNegative time.Duration) error {

	cache.lock.Lock()
	defer cache.lock.Unlock()

	if capacity != 0 {
		if cache.numEntries > capacity {
			return ErrNumOfEntriesBiggerThanCapacity
		}
		cache.capacity = capacity
	}

	if ttl != 0 && ttlForNegative != 0 {
		var ttlToAdd time.Duration
		for it := cache.NewCacheIt(); it.HasCurr(); it.Next() {
			entry := it.GetCurr()
			ttlToAdd = ttl
			if entry.state != COMPUTED { //TODO: here is possible add negative ttl in case we want add it here
				continue
			}

			if entry.state == FAILED4xx || entry.state == FAILED5XXMISSHANDLERERROR || entry.state == FAILED5xx {
				ttlToAdd = ttlForNegative
			}

			entry.expirationTime = entry.timestamp.Add(ttlToAdd)

		}
		cache.ttl = ttl
		cache.ttlForNegative = ttlForNegative
	}

	return nil
}

func (cache *CacheDriver[T, K]) RetrieveValue(keyVal T) (K, error) {
	var zeroK K
	key, err := cache.toMapKey(keyVal)
	if err != nil {
		return zeroK, err
	}

	cache.lock.Lock()
	if entry, ok := cache.table[key]; ok && entry.expirationTime.After(time.Now()) {
		cache.lock.Unlock()
		entry.lock.Lock()
		defer entry.lock.Unlock()

		for entry.state == COMPUTING { // this guard is for protection; it should never be true
			entry.cond.Wait() // it will wake up when result arrives
		}

		if entry.state != AVAILABLE {
			answer := entry.postProcessedResponse
			if cache.toCompress {
				buf, err := cache.compressor.Decompress(entry.postProcessedResponseCompressed)
				if err != nil {
					return zeroK, err
				}

				answer, err = cache.transformer.BytesToValue(buf)
				if err != nil {
					return zeroK, err
				}

			}
			return answer, nil
		}

		return zeroK, nil

	}

	cache.lock.Unlock()

	return zeroK, nil
}

// Contains returns true if the key is in the cache
func (cache *CacheDriver[T, K]) Contains(keyVal T) (bool, error) {
	key, err := cache.toMapKey(keyVal)
	if err != nil {
		return false, err
	}
	cache.lock.Lock()

	if entry, ok := cache.table[key]; ok && entry.expirationTime.After(time.Now()) {
		cache.lock.Unlock()
		entry.lock.Lock()
		defer entry.lock.Unlock()

		for entry.state == COMPUTING { // this guard is for protection; it should never be true
			entry.cond.Wait() // it will wake up when result arrives
		}

		if entry.state != AVAILABLE {
			return true, nil
		}

		return false, nil

	}

	cache.lock.Unlock()

	return false, nil
}

// testing
// Add other in retrieve from cache or compute
func (cache *CacheDriver[T, K]) StoreOrUpdate(keyVal T, newValue K) error {

	key, err := cache.toMapKey(keyVal)
	if err != nil {
		return err
	}

	cache.lock.Lock()

	if entry, ok := cache.table[key]; ok {
		cache.hitCount++
		cache.lock.Unlock()

		entry.lock.Lock()
		defer entry.lock.Unlock()

		currentTime := time.Now()
		if entry.expirationTime.Before(currentTime) {
			return ErrEntryExpired
		}

		if entry.state != COMPUTING && entry.state != AVAILABLE {
			if cache.toCompress {
				buf, err := cache.transformer.ValueToBytes(newValue)
				if err != nil {
					return err
				}
				lz4Buf, err := cache.compressor.Compress(buf)
				if err != nil {
					return err
				}
				entry.postProcessedResponseCompressed = lz4Buf
			} else {
				entry.postProcessedResponse = newValue
			}

			entry.timestamp = currentTime
			entry.expirationTime = currentTime.Add(cache.ttl)

			cache.lock.Lock()
			cache.becomeMru(entry)
			cache.lock.Unlock()
			return nil
		}

		if entry.state == AVAILABLE {
			return ErrEntryAvailableState
		}

		return ErrEntryComputingState
	}

	currTime := time.Now()
	entry, err := cache.allocateEntry(key, currTime)

	if err != nil {
		cache.lock.Unlock() // an error getting cache entry ==> we invoke directly the uservice
		// return cache.callUServices(request, payload, other...)
		return err
	}

	entry.state = COMPUTING

	//TODO specify if increases this one
	cache.missCount++
	cache.lock.Unlock() // release global lock before to take the entry lock

	entry.lock.Lock() // other requests will wait for until postProcessedResponse is gotten
	defer entry.lock.Unlock()

	retVal := newValue
	entry.state = COMPUTED

	if cache.toCompress {
		buf, err := cache.transformer.ValueToBytes(retVal) // transforms retVal into a []byte ready for compression
		if err != nil {
			entry.state = FAILED5xx
		}
		lz4Buf, err := cache.compressor.Compress(buf)
		if err != nil {
			entry.state = FAILED5xx
			entry.postProcessedResponse = retVal
		} else {
			entry.postProcessedResponseCompressed = lz4Buf
		}
		return nil
	}

	entry.postProcessedResponse = retVal
	entry.cond.Broadcast() // wake up eventual requests waiting for the result (which has failed!)
	return nil

}
