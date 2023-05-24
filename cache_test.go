package gw_cache

import (
	"encoding/json"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type RequestEntry struct {
	N        int
	Time     time.Time
	PutValue string
}

type URequest struct {
	Request  *RequestEntry
	PutValue string
}
type MyProccessor struct {
	putValue string
}

type UResponse struct {
	Urequest *URequest
}

type Response struct {
	Uresponse *UResponse
	Poem      string
}

func (p *MyProccessor) ToMapKey(entry *RequestEntry) (string, error) {
	b, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (p *MyProccessor) CallUServices(request *RequestEntry) ([]byte, *RequestError) {

	entry := request
	urequest := &URequest{
		Request:  entry,
		PutValue: entry.PutValue + "-" + p.putValue,
	}

	uresponse := &UResponse{Urequest: urequest}
	response := &Response{
		Uresponse: uresponse,
	}
	b, err := json.Marshal(*response)
	if err != nil {
		return nil, &RequestError{
			Error: err,
			Code:  Status5xx,
		}
	}
	return b, nil
}
func TestNew(t *testing.T) {

	mp := &MyProccessor{}
	cache := New[*RequestEntry, []byte](
		100,
		.4,
		time.Minute,
		mp,
	)

	assert.Equal(t, 100, cache.capacity)
	assert.Equal(t, time.Minute, cache.ttl)
	assert.Equal(t, 0, cache.hitCount)
	assert.Equal(t, 0, cache.missCount)
	assert.Equal(t, 0, cache.numEntries)
	assert.Less(t, cache.capacity, cache.extendedCapacity)
}

func TestBadFactor(t *testing.T) {

	mp := &MyProccessor{}
	assert.Panics(t, func() {
		New[*RequestEntry, []byte](100, .099, time.Minute, mp)
	})

	assert.Panics(t, func() {
		New[*RequestEntry, []byte](100, 3.00001, time.Minute, mp)
	})
}

const Capacity = 31
const TTL = 15 * time.Second

func TestWithCompress(t *testing.T) {

	transformer := NewMockTransformerI[any](t)
	proccessor := NewMockProccessorI[any, any](t)
	compressor := NewMockCompressorI(t)

	proccessor.EXPECT().ToMapKey(mock.Anything).Return("Keats", nil).Times(1)
	proccessor.EXPECT().CallUServices(mock.Anything).Return(keats, nil).Times(1)
	transformer.EXPECT().ValueToBytes(keats).Return([]byte(keats), nil).Times(1)
	compressedResponse := []byte("compressed")
	compressor.EXPECT().Compress([]byte(keats)).Return(compressedResponse, nil).Times(1)

	cache := NewWithCompression[any, any](Capacity, .4, 3*time.Minute, proccessor, transformer)
	cache.compressor = compressor

	val, ptr := cache.RetrieveFromCacheOrCompute("Keats")
	assert.Nil(t, ptr)
	assert.Equal(t, val, keats)

	proccessor.EXPECT().ToMapKey(mock.Anything).Return("Keats", nil).Times(1)
	compressor.EXPECT().Decompress(compressedResponse).Return([]byte(keats), nil).Times(1)
	transformer.EXPECT().BytesToValue([]byte(keats)).Return(keats, nil).Times(1)

	val, ptr = cache.RetrieveFromCacheOrCompute("Keats")
	assert.Nil(t, ptr)
	assert.Equal(t, val, keats)
}

func insertEntry[T any](
	cache *CacheDriver[T, T],
	proccessor *MockProccessorI[T, T],
	request T,
) (T, *RequestError) {
	s, _ := json.Marshal(request)
	proccessor.EXPECT().ToMapKey(request).Return(string(s), nil).Times(1)
	proccessor.EXPECT().CallUServices(request).Return(request, nil).Times(1)

	return cache.RetrieveFromCacheOrCompute(request)
}

func createCacheWithCapEntriesInside(
	capacity int,
	proccessor *MockProccessorI[*RequestEntry, *RequestEntry],
) (*CacheDriver[*RequestEntry, *RequestEntry], map[*RequestEntry]bool) {

	cache := New[*RequestEntry, *RequestEntry](capacity, .4, TTL, proccessor)

	requestTbl := make(map[*RequestEntry]bool)
	for i := 0; i < capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}
		insertEntry(cache, proccessor, request)
		requestTbl[request] = true
	}

	return cache, requestTbl
}

func createCompressedCacheWithCapEntriesInside(
	capacity int,
	proccessor *MockProccessorI[*RequestEntry, *RequestEntry],
) (*CacheDriver[*RequestEntry, *RequestEntry], map[*RequestEntry]bool) {

	transformer := &DefaultTransformer[*RequestEntry]{}
	cache := NewWithCompression[*RequestEntry, *RequestEntry](capacity, .4, TTL, proccessor, transformer)

	requestTbl := make(map[*RequestEntry]bool)
	for i := 0; i < capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}
		insertEntry(cache, proccessor, request)
		requestTbl[request] = true
	}

	return cache, requestTbl
}

func TestEvictions(t *testing.T) {

	proccessor := NewMockProccessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(Capacity, proccessor)

	// now we insert Capacity new entries which should evict all the previously inserted ones
	for i := Capacity; i < 2*Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}
		b, requestError := insertEntry(cache, proccessor, request)
		assert.Nil(t, requestError)
		assert.NotNil(t, b)
	}

	// now we verify that entries en tbl are not in the cache
	for req := range tbl {
		proccessor.EXPECT().ToMapKey(req).Return(strconv.Itoa(req.Time.Nanosecond()), nil).Times(1)
		assert.False(t, cache.has(req))
	}
}

func TestCacheDriver_Has(t *testing.T) {

	proccessor := NewMockProccessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(Capacity, proccessor)

	for req := range tbl {
		s, _ := json.Marshal(req)
		proccessor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		assert.True(t, cache.has(req))
	}
}

func TestLRUOrder(t *testing.T) {

	proccessor := NewMockProccessorI[*RequestEntry, *RequestEntry](t)
	cache, _ := createCacheWithCapEntriesInside(
		Capacity,
		proccessor,
	)

	it := cache.NewCacheIt()
	prevTimeStamp := it.GetCurr().timestamp
	for it.Next(); it.HasCurr(); it.Next() {
		curr := it.GetCurr()
		assert.True(t, prevTimeStamp.After(curr.timestamp))
		prevTimeStamp = curr.timestamp
	}
}

func TestCacheDriver_testTTL(t *testing.T) {

	ttl := 3 * time.Second
	proccessor := NewMockProccessorI[any, any](t)
	cache := New[any, any](Capacity, .4, ttl, proccessor)

	request := &RequestEntry{
		N:    10,
		Time: time.Now(),
	}

	proccessor.EXPECT().ToMapKey(request).Return(strconv.Itoa(request.Time.Nanosecond()), nil).Times(2)
	proccessor.EXPECT().CallUServices(request).Return(request, nil).Times(1)
	b, requestError := cache.RetrieveFromCacheOrCompute(request)
	assert.Nil(t, requestError)
	assert.NotNil(t, b)

	time.Sleep(ttl) // wait for tt expiration

	assert.False(t, cache.has(request))
}

func TestRandomTouches(t *testing.T) {
	proccessor := NewMockProccessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		2,
		proccessor,
	)

	N := len(tbl)
	requests := make([]*RequestEntry, N)
	i := 0
	for req := range tbl {
		requests[i] = req
		i++
	}

	// var response *RequestEntry
	for i := range rand.Perm(N) {
		req := requests[i]
		s, _ := json.Marshal(req)
		proccessor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		b, requestError := cache.RetrieveFromCacheOrCompute(req)
		assert.Nil(t, requestError)

		assert.Equal(t, cache.getMru().postProcessedResponse, b)
	}
}

func TestCompressRandomTouches(t *testing.T) {
	proccessor := NewMockProccessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCompressedCacheWithCapEntriesInside(
		2,
		proccessor,
	)

	N := len(tbl)
	requests := make([]*RequestEntry, N)
	i := 0
	for req := range tbl {
		requests[i] = req
		i++
	}

	// var response *RequestEntry
	for i := range rand.Perm(N) {
		req := requests[i]
		s, _ := json.Marshal(req)
		proccessor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		b, requestError := cache.RetrieveFromCacheOrCompute(req)
		assert.Nil(t, requestError)

		decompressedReponse, _ := cache.compressor.Decompress(cache.getMru().postProcessedResponseCompressed)
		data, _ := cache.transformer.BytesToValue(decompressedReponse)
		assert.Equal(t, data, b)
	}
}

func TestCacheDriver_CacheState(t *testing.T) {

	proccessor := NewMockProccessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		2,
		proccessor,
	)
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	// some random touches
	for i := 0; i < 100; i++ {
		i := rand.Intn(N)
		req := requests[i]
		s, _ := json.Marshal(req)
		proccessor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		_, _ = cache.RetrieveFromCacheOrCompute(req)
	}

	state, err := cache.GetState()
	assert.Nil(t, err)
	assert.NotNil(t, state)

}

func TestCacheDriver_Clean(t *testing.T) {

	proccessor := NewMockProccessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		Capacity,
		proccessor,
	)
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	// some random touches
	for i := 0; i < 100; i++ {
		i := rand.Intn(N)
		req := requests[i]
		s, _ := json.Marshal(req)
		proccessor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		_, _ = cache.RetrieveFromCacheOrCompute(req)
	}

	err := cache.Clean()
	assert.Nil(t, err)
	assert.Equal(t, 0, cache.missCount)
	assert.Equal(t, 0, cache.hitCount)
	assert.Equal(t, 0, cache.numEntries)
	assert.Equal(t, Capacity, cache.capacity)
	assert.Equal(t, TTL, cache.ttl)

	state, _ := cache.GetState()

	var s CacheState
	err = json.Unmarshal([]byte(state), &s)

	assert.Equal(t, 0, s.NumEntries)
	assert.Equal(t, 0, s.HitCount)
	assert.Equal(t, 0, s.MissCount)
}

func TestCacheDriver_HitCost(t *testing.T) {
	// proccessor := NewMockProccessorI[*RequestEntry, *RequestEntry](t)
	processor := &MyProccessor{}
	cache := New[*RequestEntry, []byte](Capacity, .4, TTL, processor)

	requestTbl := make(map[*RequestEntry]bool)
	for i := 0; i < Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}
		cache.RetrieveFromCacheOrCompute(request)
		requestTbl[request] = true
	}
	// 	1,
	// 	proccessor,
	// )
	N := len(requestTbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range requestTbl {
		requests = append(requests, req)
	}

	// some random touches
	for i := 0; i < 1000000; i++ {
		req := requests[0]
		_, err := cache.RetrieveFromCacheOrCompute(req)
		assert.Nil(t, err)
	}
}

func TestConcurrency(t *testing.T) {

	const ConcurrencyLevel = 20
	const SuperCap = 3037
	const NumRepeatedCalls = 50

	myProccessor := &MyProccessor{}
	cache := New[*RequestEntry, []byte](SuperCap, .3, 30*time.Second, myProccessor)

	tbl := make(map[*RequestEntry]bool)
	for i := 0; i < Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}

		_, _ = cache.RetrieveFromCacheOrCompute(request)
		tbl[request] = true
	}

	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	for i := 0; i < 1e4; i++ {
		wg := sync.WaitGroup{}
		wg.Add(ConcurrencyLevel)
		for k := 0; k < ConcurrencyLevel; k++ {

			go func() { // goroutine emulates an avalanche of repeated requests

				idx := rand.Intn(N) // choose request randomly
				req := requests[idx]

				// now we simulate the avalanche
				for j := 0; j < NumRepeatedCalls; j++ {

					go func() {
						_, requestError := cache.RetrieveFromCacheOrCompute(req)
						assert.Nil(t, requestError)

						// var response Response
						// err := json.Unmarshal(b.([]byte), &response)
						// assert.Nil(t, err)
					}()

				}

				wg.Done()
			}()

		}
		wg.Wait()
	}
}

func TestConcurrencyAndCompress(t *testing.T) {

	const ConcurrencyLevel = 10
	const SuperCap = 1019
	const NumRepeatedCalls = 20

	myProccessor := &MyProccessor{}
	defaultTransformer := &DefaultTransformer[[]byte]{}
	cache := NewWithCompression[*RequestEntry, []byte](
		SuperCap,
		.3,
		30*time.Second,
		myProccessor,
		defaultTransformer,
	)

	tbl := make(map[*RequestEntry]bool)
	for i := 0; i < Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}

		_, _ = cache.RetrieveFromCacheOrCompute(request)
		tbl[request] = true
	}

	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	for i := 0; i < 1e3; i++ {
		wg := sync.WaitGroup{}
		wg.Add(ConcurrencyLevel)
		for k := 0; k < ConcurrencyLevel; k++ {

			go func() { // goroutine emulates an avalanche of repeated requests

				idx := rand.Intn(N) // choose request randomly
				req := requests[idx]

				// now we simulate the avalanche
				for j := 0; j < NumRepeatedCalls; j++ {

					go func(request *RequestEntry) {
						_, requestError := cache.RetrieveFromCacheOrCompute(req)
						assert.Nil(t, requestError)

						// ref := response.(*Response)
						// assert.Equal(t, ref.Uresponse.Urequest.Request.N, request.N)
						// assert.Equal(t, ref.Uresponse.Urequest.Request.PutValue, request.PutValue)
						// assert.Equal(t, ref.Poem, keats)
					}(req)
				}

				wg.Done()
			}()

		}
		wg.Wait()
	}
}

// func TestCacheDriver_LazyRemove(t *testing.T) {

// 	cache, tbl := createCacheWithCapEntriesInside()
// 	N := len(tbl)
// 	requests := make([]*RequestEntry, 0, N)
// 	for req := range tbl {
// 		requests = append(requests, req)
// 	}

// 	var lastRequest *RequestEntry
// 	for i := 0; i < 100; i++ {
// 		i := rand.Intn(N)
// 		req := requests[i]
// 		lastRequest = req
// 		_, _ = cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
// 		isMru, err := cache.isKeyMru(req)
// 		assert.Nil(t, err)
// 		assert.True(t, isMru)
// 	}

// 	err := cache.LazyRemove(lastRequest)
// 	assert.Nil(t, err)
// 	assert.False(t, cache.has(lastRequest))
// }

// func TestCacheDriver_Contains(t *testing.T) {

// 	cache, tbl := createCacheWithCapEntriesInside()
// 	N := len(tbl)
// 	requests := make([]*RequestEntry, 0, N)
// 	for req := range tbl {
// 		requests = append(requests, req)
// 	}

// 	for i := 0; i < N; i++ {
// 		req := requests[i]
// 		_, _ = cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
// 	}

// 	for i := 0; i < N; i++ {
// 		req := requests[i]
// 		ok, err := cache.Contains(req)
// 		assert.Nil(t, err)
// 		assert.True(t, ok)
// 	}
// }

// func TestCacheDriver_Touch(t *testing.T) {

// 	cache, tbl := createCacheWithCapEntriesInside()
// 	N := len(tbl)
// 	requests := make([]*RequestEntry, 0, N)
// 	for req := range tbl {
// 		requests = append(requests, req)
// 	}

// 	for i := 0; i < N; i++ {
// 		req := requests[i]
// 		_, _ = cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
// 	}

// 	for i := 0; i < N; i++ {
// 		req := requests[i]
// 		err := cache.Touch(req)
// 		assert.Nil(t, err)

// 		mru, err := cache.isKeyMru(req)

// 		assert.Nil(t, err)
// 		assert.True(t, mru)
// 	}
// }
