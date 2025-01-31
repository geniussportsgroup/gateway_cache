package gw_cache

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/geniussportsgroup/gateway_cache/v3/mocks"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
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
type MyProcessor struct {
	putValue string
}

type UResponse struct {
	Urequest *URequest
}

type Response struct {
	Uresponse *UResponse
	Poem      string
}

func (p *MyProcessor) ToMapKey(entry *RequestEntry) (string, error) {
	b, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (p *MyProcessor) CacheMissSolver(request *RequestEntry,
	other ...interface{}) ([]byte, *RequestError) {

	entry := request
	urequest := &URequest{
		Request:  entry,
		PutValue: entry.PutValue + "-" + p.putValue,
	}

	uresponse := &UResponse{Urequest: urequest}
	response := &Response{
		Uresponse: uresponse,
		Poem:      keats,
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

	mp := &MyProcessor{}
	cache := New[*RequestEntry, []byte](
		100,
		.4,
		time.Minute,
		time.Minute,
		mp.CacheMissSolver,
		mp.ToMapKey,
	)

	assert.Equal(t, 100, cache.capacity)
	assert.Equal(t, time.Minute, cache.ttl)
	assert.Equal(t, 0, cache.hitCount)
	assert.Equal(t, 0, cache.missCount)
	assert.Equal(t, 0, cache.numEntries)
	assert.Less(t, cache.capacity, cache.extendedCapacity)
}

func TestBadFactor(t *testing.T) {

	mp := &MyProcessor{}
	assert.Panics(t, func() {
		New[*RequestEntry, []byte](100, .099, time.Minute, time.Minute, mp.CacheMissSolver, mp.ToMapKey)
	})

	assert.Panics(t, func() {
		New[*RequestEntry, []byte](100, 3.00001, time.Minute, time.Minute, mp.CacheMissSolver, mp.ToMapKey)
	})
}

const Capacity = 31
const TTL = 15 * time.Second

func TestWithCompress(t *testing.T) {

	transformer := mocks.NewTransformerI[any](t)
	processor := NewMockProcessorI[any, any](t)
	compressor := mocks.NewCompressorI(t)

	processor.EXPECT().ToMapKey(mock.Anything).Return("Keats", nil).Times(1)
	processor.EXPECT().CacheMissSolver(mock.Anything).Return(keats, nil).Times(1)
	transformer.EXPECT().ValueToBytes(keats).Return([]byte(keats), nil).Times(1)
	compressedResponse := []byte("compressed")
	compressor.EXPECT().Compress([]byte(keats)).Return(compressedResponse, nil).Times(1)

	cache := NewWithCompression[any, any](
		Capacity,
		.4,
		3*time.Minute,
		20*time.Second,
		processor.CacheMissSolver,
		processor.ToMapKey,
		transformer)
	cache.compressor = compressor

	val, ptr := cache.RetrieveFromCacheOrCompute("Keats")
	assert.Nil(t, ptr)
	assert.Equal(t, val, keats)

	processor.EXPECT().ToMapKey(mock.Anything).Return("Keats", nil).Times(1)
	compressor.EXPECT().Decompress(compressedResponse).Return([]byte(keats), nil).Times(1)
	transformer.EXPECT().BytesToValue([]byte(keats)).Return(keats, nil).Times(1)

	val, ptr = cache.RetrieveFromCacheOrCompute("Keats")
	assert.Nil(t, ptr)
	assert.Equal(t, val, keats)
}

func insertEntry[T any](
	cache *CacheDriver[T, T],
	processor *MockProcessorI[T, T],
	request T,
) (T, *RequestError) {
	s, _ := json.Marshal(request)
	processor.EXPECT().ToMapKey(request).Return(string(s), nil).Times(1)
	processor.EXPECT().CacheMissSolver(request).Return(request, nil).Times(1)

	return cache.RetrieveFromCacheOrCompute(request)
}

func createCacheWithCapEntriesInside(
	capacity int,
	processor *MockProcessorI[*RequestEntry, *RequestEntry],
) (*CacheDriver[*RequestEntry, *RequestEntry], map[*RequestEntry]bool) {

	cache := New[*RequestEntry, *RequestEntry](capacity, .4, TTL, TTL, processor.CacheMissSolver, processor.ToMapKey)

	requestTbl := make(map[*RequestEntry]bool)
	for i := 0; i < capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}
		insertEntry(cache, processor, request)
		requestTbl[request] = true
	}

	return cache, requestTbl
}

func createCompressedCacheWithCapEntriesInside(
	capacity int,
	processor *MockProcessorI[*RequestEntry, *RequestEntry],
) (*CacheDriver[*RequestEntry, *RequestEntry], map[*RequestEntry]bool) {

	transformer := &DefaultTransformer[*RequestEntry]{}
	cache := NewWithCompression[*RequestEntry, *RequestEntry](capacity, .4, TTL, TTL,
		processor.CacheMissSolver, processor.ToMapKey, transformer)

	requestTbl := make(map[*RequestEntry]bool)
	for i := 0; i < capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}
		insertEntry(cache, processor, request)
		requestTbl[request] = true
	}

	return cache, requestTbl
}

func TestEvictions(t *testing.T) {

	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(Capacity, processor)

	// now we insert Capacity new entries which should evict all the previously inserted ones
	for i := Capacity; i < 2*Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}
		b, requestError := insertEntry(cache, processor, request)
		assert.Nil(t, requestError)
		assert.NotNil(t, b)
	}

	// now we verify that entries en tbl are not in the cache
	for req := range tbl {
		processor.EXPECT().ToMapKey(req).Return(strconv.Itoa(req.Time.Nanosecond()), nil).Times(1)
		assert.False(t, cache.has(req))
	}
}

func TestCacheDriver_Has(t *testing.T) {

	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(Capacity, processor)

	for req := range tbl {
		s, _ := json.Marshal(req)
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		assert.True(t, cache.has(req))
	}
}

func TestLRUOrder(t *testing.T) {

	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, _ := createCacheWithCapEntriesInside(
		Capacity,
		processor,
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
	processor := NewMockProcessorI[any, any](t)
	cache := New[any, any](Capacity, .4, ttl, ttl, processor.CacheMissSolver, processor.ToMapKey)

	request := &RequestEntry{
		N:    10,
		Time: time.Now(),
	}

	processor.EXPECT().ToMapKey(request).Return(strconv.Itoa(request.Time.Nanosecond()), nil).Times(2)
	processor.EXPECT().CacheMissSolver(request).Return(request, nil).Times(1)
	b, requestError := cache.RetrieveFromCacheOrCompute(request)
	assert.Nil(t, requestError)
	assert.NotNil(t, b)

	time.Sleep(ttl) // wait for tt expiration

	assert.False(t, cache.has(request))
}

func TestRandomTouches(t *testing.T) {
	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		2,
		processor,
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
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		b, requestError := cache.RetrieveFromCacheOrCompute(req)
		assert.Nil(t, requestError)

		assert.Equal(t, cache.getMru().postProcessedResponse, b)
	}
}

func TestCompressRandomTouches(t *testing.T) {
	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCompressedCacheWithCapEntriesInside(
		2,
		processor,
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
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		b, requestError := cache.RetrieveFromCacheOrCompute(req)
		assert.Nil(t, requestError)

		decompressedReponse, _ := cache.compressor.Decompress(cache.getMru().postProcessedResponseCompressed)
		data, _ := cache.transformer.BytesToValue(decompressedReponse)
		assert.Equal(t, data, b)
	}
}

func TestCacheDriver_CacheState(t *testing.T) {

	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		2,
		processor,
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
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		_, _ = cache.RetrieveFromCacheOrCompute(req)
	}

	state, err := cache.GetState()
	assert.Nil(t, err)
	assert.NotNil(t, state)

}

func TestCacheDriver_Clean(t *testing.T) {

	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		Capacity,
		processor,
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
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
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
	processor := &MyProcessor{}
	cache := New[*RequestEntry, []byte](Capacity, .4, TTL, TTL, processor.CacheMissSolver, processor.ToMapKey)

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

	myProccessor := &MyProcessor{}
	cache := New[*RequestEntry, []byte](
		SuperCap,
		.3,
		30*time.Second,
		30*time.Second,
		myProccessor.CacheMissSolver,
		myProccessor.ToMapKey,
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

	for i := 0; i < 1e4; i++ {
		wg := sync.WaitGroup{}
		wg.Add(ConcurrencyLevel)
		for k := 0; k < ConcurrencyLevel; k++ {

			go func() { // goroutine emulates an avalanche of repeated requests

				idx := rand.Intn(N) // choose request randomly
				req := requests[idx]

				// now we simulate the avalanche
				for j := 0; j < NumRepeatedCalls; j++ {

					go func(req *RequestEntry) {
						_, requestError := cache.RetrieveFromCacheOrCompute(req)
						assert.Nil(t, requestError)

						// var response Response
						// err := json.Unmarshal(b.([]byte), &response)
						// assert.Nil(t, err)
					}(req)

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

	myProccessor := &MyProcessor{}
	defaultTransformer := &DefaultTransformer[[]byte]{}
	cache := NewWithCompression[*RequestEntry, []byte](
		SuperCap,
		.3,
		30*time.Second,
		30*time.Second,
		myProccessor.CacheMissSolver,
		myProccessor.ToMapKey,
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

					go func(req *RequestEntry) {
						response, requestError := cache.RetrieveFromCacheOrCompute(req)
						assert.Nil(t, requestError)

						ref := &Response{}
						err := json.Unmarshal(response, &ref)
						assert.Nil(t, err)
						assert.Equal(t, ref.Uresponse.Urequest.Request.N, req.N)
						assert.Equal(t, ref.Poem, keats)
					}(req)
				}

				wg.Done()
			}()

		}
		wg.Wait()
	}
}

func TestCacheDriver_LazyRemove(t *testing.T) {

	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		Capacity,
		processor,
	)
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	var lastRequest *RequestEntry
	for i := 0; i < 100; i++ {
		i := rand.Intn(N)
		req := requests[i]
		lastRequest = req
		s, _ := json.Marshal(req)
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(2)
		_, _ = cache.RetrieveFromCacheOrCompute(req)
		isMru, err := cache.isKeyMru(req)
		assert.Nil(t, err)
		assert.True(t, isMru)
	}

	s, _ := json.Marshal(lastRequest)
	processor.EXPECT().ToMapKey(lastRequest).Return(string(s), nil).Times(2)
	err := cache.LazyRemove(lastRequest)
	assert.Nil(t, err)
	assert.False(t, cache.has(lastRequest))
}

func TestCacheDriver_Contains(t *testing.T) {

	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		Capacity,
		processor,
	)
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	for i := 0; i < N; i++ {
		req := requests[i]
		s, _ := json.Marshal(req)
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		_, _ = cache.RetrieveFromCacheOrCompute(req)
	}

	for i := 0; i < N; i++ {
		req := requests[i]
		s, _ := json.Marshal(req)
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(1)
		ok, err := cache.Contains(req)
		assert.Nil(t, err)
		assert.True(t, ok)
	}
}

func TestCacheDriver_Touch(t *testing.T) {

	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache, tbl := createCacheWithCapEntriesInside(
		Capacity,
		processor,
	)
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	for i := 0; i < N; i++ {
		req := requests[i]
		s, _ := json.Marshal(req)
		processor.EXPECT().ToMapKey(req).Return(string(s), nil).Times(2)
		err := cache.Touch(req)
		assert.Nil(t, err)

		mru, err := cache.isKeyMru(req)

		assert.Nil(t, err)
		assert.True(t, mru)
	}
}

// test the StoreValue method first insert values in the cache
func TestCacheDriver_StoreOrUpdateValue(t *testing.T) {
	processor := NewMockProcessorI[int, int](t)
	cache := New[int, int](Capacity, .4, TTL, TTL, processor.CacheMissSolver, processor.ToMapKey)

	elements := Capacity
	for i := 0; i < elements; i++ {
		processor.EXPECT().ToMapKey(i).Return(fmt.Sprint(i), nil).Times(2)
		err := cache.StoreOrUpdate(i, i)
		assert.Nil(t, err)
		err = cache.StoreOrUpdate(i, i)
		assert.Nil(t, err)
	}

	for i := 0; i < elements; i++ {
		processor.EXPECT().ToMapKey(i).Return(fmt.Sprint(i), nil).Times(1)
		val, err := cache.RetrieveFromCacheOrCompute(i)

		assert.Equal(t, val, i)
		assert.Nil(t, err)

	}
}

func TestCacheDriver_StoreValueConcurrentInsert(t *testing.T) {
	processor := NewMockProcessorI[int, int](t)
	cache := New[int, int](Capacity, .4, TTL, TTL, processor.CacheMissSolver, processor.ToMapKey)

	elements := Capacity
	goroutines := 5
	wg := sync.WaitGroup{}
	wg.Add(goroutines * elements)
	for i := 0; i < elements; i++ {
		processor.EXPECT().ToMapKey(i).Return(fmt.Sprint(i), nil).Times(goroutines)
		for j := 0; j < goroutines; j++ {
			go func(i int, t *testing.T, wg *sync.WaitGroup) {
				err := cache.StoreOrUpdate(i, i)
				assert.Nil(t, err)
				wg.Done()
			}(i, t, &wg)
		}
	}
	wg.Wait()

	for i := 0; i < elements; i++ {
		processor.EXPECT().ToMapKey(i).Return(fmt.Sprint(i), nil).Times(1)
		val, err := cache.RetrieveFromCacheOrCompute(i)

		assert.Equal(t, val, i)
		assert.Nil(t, err)

	}
}

// test retrieve value
func TestCacheDriver_RetrieveValue(t *testing.T) {
	processor := NewMockProcessorI[int, int](t)
	cache := New[int, int](Capacity, .4, TTL, TTL, processor.CacheMissSolver, processor.ToMapKey)

	elements := Capacity
	for i := 0; i < elements; i++ {
		b, requestError := insertEntry(cache, processor, i)
		assert.Nil(t, requestError)
		assert.NotNil(t, b)
	}

	for i := 0; i < elements; i++ {
		processor.EXPECT().ToMapKey(i).Return(fmt.Sprint(i), nil).Times(1)
		val, err := cache.RetrieveValue(i)

		assert.Equal(t, val, i)
		assert.Nil(t, err)

	}
}

func TestTTLForNegative(t *testing.T) {

	assert := assert.New(t)
	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	cache := New[*RequestEntry, *RequestEntry](
		Capacity,
		.4,
		2*time.Second,
		time.Second,
		processor.CacheMissSolver,
		processor.ToMapKey,
	)

	//Add a negative entry

	negativePayload := &RequestEntry{}
	processor.EXPECT().ToMapKey(negativePayload).Return("Keats", nil).Times(4)
	processor.EXPECT().CacheMissSolver(negativePayload).Return(nil, &RequestError{
		Code: Status5xx,
	}).Times(1)

	//negative assertions
	_, requestErr := cache.RetrieveFromCacheOrCompute(negativePayload)
	assert.NotNil(requestErr)

	normalPayload := &RequestEntry{N: 1}

	processor.EXPECT().ToMapKey(normalPayload).Return("Keats1", nil).Times(4)
	processor.EXPECT().CacheMissSolver(normalPayload).Return(nil, nil).Times(1)

	_, requestErr = cache.RetrieveFromCacheOrCompute(normalPayload)
	assert.Nil(requestErr)

	contained, err := cache.Contains(negativePayload)
	assert.Nil(err)
	assert.True(contained)

	contained, err = cache.Contains(normalPayload)
	assert.Nil(err)
	assert.True(contained)

	time.Sleep(1 * time.Second)
	contained, err = cache.Contains(negativePayload)
	assert.Nil(err)
	assert.False(contained)

	contained, err = cache.Contains(normalPayload)
	assert.Nil(err)
	assert.True(contained)

	time.Sleep(1 * time.Second)

	contained, err = cache.Contains(negativePayload)
	assert.Nil(err)
	assert.False(contained)

	contained, err = cache.Contains(normalPayload)
	assert.Nil(err)
	assert.False(contained)
}

type Other struct{}

func (p *Other) ToMapKey(entry string) (string, error) {
	return entry, nil
}

func (p *Other) CacheMissSolver(entry string, other ...interface{}) (string, *RequestError) {
	response, ok := other[0].(string)
	if !ok {
		return "", &RequestError{
			Code: Status5xx,
		}
	}
	return response, nil
}

func TestOtherInCache(t *testing.T) {
	assert := assert.New(t)
	p := &Other{}
	cache := New[string, string](
		Capacity,
		.4,
		2*time.Second,
		time.Second,
		p.CacheMissSolver,
		p.ToMapKey,
	)
	value, err := cache.RetrieveFromCacheOrCompute("1", "Keats")
	assert.Nil(err)
	assert.Equal("Keats", value)

	value, err = cache.RetrieveFromCacheOrCompute("2", 0)
	assert.NotNil(err)
	assert.Equal(err.Code, Status5xx)
	assert.Equal("", value)

}

type MockReporter struct {
	missCount chan int
	hitCount  chan int
}

func (m *MockReporter) ReportMiss() {
	m.missCount <- 1
}

func (m *MockReporter) ReportHit() {
	m.hitCount <- 1
}

func TestReporter(t *testing.T) {

	assert := assert.New(t)
	processor := NewMockProcessorI[*RequestEntry, *RequestEntry](t)
	reporter := &MockReporter{
		missCount: make(chan int, 1),
		hitCount:  make(chan int, 1),
	}
	cache := New[*RequestEntry, *RequestEntry](
		Capacity,
		.4,
		2*time.Second,
		time.Second,
		processor.CacheMissSolver,
		processor.ToMapKey,
	)

	cache.SetReporter(reporter)

	processor.EXPECT().ToMapKey(mock.Anything).Return("Keats", nil).Times(1)
	processor.EXPECT().CacheMissSolver(mock.Anything).Return(nil, nil).Times(1)
	_, err := cache.RetrieveFromCacheOrCompute(&RequestEntry{})
	assert.Nil(err)
	missCount := <-reporter.missCount
	assert.Equal(1, missCount)

	processor.EXPECT().ToMapKey(mock.Anything).Return("Keats", nil).Times(1)
	_, err = cache.RetrieveFromCacheOrCompute(&RequestEntry{})
	assert.Nil(err)
	hitCount := <-reporter.hitCount
	assert.Equal(1, hitCount)

}

type BenchProcessor struct{}

type Adder struct {
	num1, num2 int
}

func (p *BenchProcessor) ToMapKey(adder Adder) (string, error) {
	return fmt.Sprintf("%d+%d", adder.num1, adder.num2), nil
}

func (p *BenchProcessor) CacheMissSolver(adder Adder, _ ...interface{}) (int, *RequestError) {
	return adder.num1 + adder.num2, nil
}

var seed int64 = 39823823434

func BenchmarkInsertStatic(b *testing.B) {
	benchInsert(b, seed)
}
func BenchmarkInsertDynamic(b *testing.B) {
	benchInsert(b, time.Now().Unix())
}

func benchInsert(b *testing.B, seed int64) {

	bp := &BenchProcessor{}
	cache := New[Adder, int](Capacity, 0.5, TTL, TTL, bp.CacheMissSolver, bp.ToMapKey)
	rand.Seed(seed)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			num1 := rand.Int()
			num2 := rand.Int()
			adder := Adder{num1, num2}
			_, _ = cache.RetrieveFromCacheOrCompute(adder)
		}
	}
}

func createRandomArray(seed int64, size int) []Adder {
	rand.Seed(seed)
	arr := make([]Adder, size)
	for i := 0; i < size; i++ {
		num1 := rand.Int()
		num2 := rand.Int()
		arr[i] = Adder{num1, num2}
	}
	return arr
}

func benchInsertAvalanche(b *testing.B, seed int64) {

	var size int = 1e3
	arr := createRandomArray(seed, size)
	bp := &BenchProcessor{}
	cache := New[Adder, int](Capacity, 0.5, TTL, TTL, bp.CacheMissSolver, bp.ToMapKey)
	rand.Seed(seed)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < size; j++ {
			for k := 0; k < 1e3; k++ {
				_, _ = cache.RetrieveFromCacheOrCompute(arr[j])
			}
		}
	}

}

func BenchmarkAvalancheStatic(b *testing.B) {
	benchInsertAvalanche(b, seed)
}

func BenchmarkAvalancheDynamic(b *testing.B) {
	benchInsertAvalanche(b, time.Now().Unix())
}
