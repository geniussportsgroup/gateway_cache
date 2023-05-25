package gw_cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var keats string = `A thing of beauty is a joy for ever:\n
Its loveliness increases; it will never\n
Pass into nothingness; but still will keep\n
A bower quiet for us, and a sleep\n
Full of sweet dreams, and health, and quiet breathing.\n
Therefore, on every morrow, are we wreathing\n
A flowery band to bind us to the earth,\n
Spite of despondence, of the inhuman dearth\n
Of noble natures, of the gloomy days,\n
Of all the unhealthy and o'er-darkened ways\n
Made for our searching: yes, in spite of all,\n
Some shape of beauty moves away the pall\n
From our dark spirits. Such the sun, the moon,\n
Trees old, and young, sprouting a shady boon\n
For simple sheep; and such are daffodils\n
With the green world they live in; and clear rills\n
That for themselves a cooling covert make\n
'Gainst the hot season; the mid forest brake,\n
Rich with a sprinkling of fair musk-rose blooms:\n
And such too is the grandeur of the dooms\n
We have imagined for the mighty dead;\n
All lovely tales that we have heard or read:\n
An endless fountain of immortal drink,\n
Pouring unto us from the heaven's brink.\n`

type RequestEntry struct {
	N        int
	Time     time.Time
	PutValue string
}

type URequest struct {
	Request  *RequestEntry
	PutValue string
}

type UResponse struct {
	Urequest *URequest
}

type Response struct {
	Uresponse *UResponse
	Poem      string
}

func toKey(entry interface{}) (string, error) {
	b, err := json.Marshal(*entry.(*RequestEntry))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func toBytes(value interface{}) ([]byte, error) {

	response := value.(*Response)
	b, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func toVal(buf []byte) (interface{}, error) {

	response := &Response{}
	err := json.Unmarshal(buf, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func preProcessRequest(request interface{}, other ...interface{}) (interface{}, *RequestError) {

	entry := request.(*RequestEntry)
	if entry.N < 0 {
		return nil, &RequestError{
			Error: errors.New("N is negative"),
			Code:  Status4xx,
		}
	}
	if entry.N < 10 {
		return nil, &RequestError{
			Error: errors.New("N must be greater or equal than 10"),
			Code:  Status5xx,
		}
	}

	entry.PutValue = other[0].(string)
	return entry, nil
}

func callServices(request, _ interface{}, other ...interface{}) (interface{}, *RequestError) {

	entry := request.(*RequestEntry)
	urequest := &URequest{
		Request:  entry,
		PutValue: entry.PutValue + "-" + other[1].(string),
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

func callServicesWithCompression(request, _ interface{}, other ...interface{}) (interface{}, *RequestError) {

	entry := request.(*RequestEntry)
	urequest := &URequest{
		Request:  entry,
		PutValue: entry.PutValue + "-" + other[1].(string),
	}

	uresponse := &UResponse{Urequest: urequest}
	response := &Response{
		Uresponse: uresponse,
		Poem:      keats,
	}

	return response, nil
}

func TestNew(t *testing.T) {

	cache := New(100, .4, time.Minute, toKey, preProcessRequest, callServices)

	assert.Equal(t, 100, cache.capacity)
	assert.Equal(t, time.Minute, cache.ttl)
	assert.Equal(t, 0, cache.hitCount)
	assert.Equal(t, 0, cache.missCount)
	assert.Equal(t, 0, cache.numEntries)
	assert.Less(t, cache.capacity, cache.extendedCapacity)
}

func TestBadFactor(t *testing.T) {

	assert.Panics(t, func() {
		New(100, .099, time.Minute, toKey, nil, callServices)
	})

	assert.Panics(t, func() {
		New(100, 3.00001, time.Minute, toKey, nil, callServices)
	})
}

const Capacity = 31
const TTL = 15 * time.Second

func createCacheWithCapEntriesInside() (*CacheDriver, map[*RequestEntry]bool) {

	cache := New(Capacity, .4, TTL, toKey, preProcessRequest, callServices)

	requestTbl := make(map[*RequestEntry]bool)
	for i := 0; i < Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}

		str := strconv.Itoa(i)
		_, _ = cache.RetrieveFromCacheOrCompute(request, "Request: "+str, "Urequest: "+str)
		requestTbl[request] = true
	}

	return cache, requestTbl
}

func createCompressedCacheWithCapEntriesInside() (*CacheDriver, map[*RequestEntry]bool) {

	cache := NewWithCompression(Capacity, .4, TTL, toKey, toBytes, toVal,
		preProcessRequest, callServicesWithCompression)

	requestTbl := make(map[*RequestEntry]bool)
	for i := 0; i < Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}

		str := strconv.Itoa(i)
		_, _ = cache.RetrieveFromCacheOrCompute(request, "Request: "+str, "Urequest: "+str)
		requestTbl[request] = true
	}

	return cache, requestTbl
}

func TestCompress(t *testing.T) {

	callFct := func(request, payload interface{}, other ...interface{}) (interface{}, *RequestError) {

		value := keats
		return value, nil
	}

	cache := NewWithCompression(Capacity, .4, 3*time.Minute,
		func(key interface{}) (string, error) { return key.(string), nil },
		func(value interface{}) ([]byte, error) { return []byte(value.(string)), nil },
		func(bytes []byte) (interface{}, error) { return string(bytes), nil },
		nil, callFct)

	val, ptr := cache.RetrieveFromCacheOrCompute("Keats")
	assert.Nil(t, ptr)
	assert.Equal(t, val, keats)

	val, ptr = cache.RetrieveFromCacheOrCompute("Keats")
	assert.Nil(t, ptr)
	assert.Equal(t, val, keats)
}

func TestCacheProcessing(t *testing.T) {

	cache := New(Capacity, .4, TTL, toKey, preProcessRequest, callServices)

	var response Response

	for i := 0; i < Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}

		str := strconv.Itoa(i)
		b, requestError := cache.RetrieveFromCacheOrCompute(request,
			"Request: "+str, "Urequest: "+str)
		assert.Nil(t, requestError)
		assert.NotNil(t, b)

		err := json.Unmarshal(b.([]byte), &response)
		assert.Nil(t, err)
		assert.Equal(t, request.N, response.Uresponse.Urequest.Request.N)
		assert.Equal(t, request.PutValue, response.Uresponse.Urequest.Request.PutValue) // fmt.Printf("%#v", response)
		assert.True(t, request.Time.Equal(response.Uresponse.Urequest.Request.Time))
	}
}

func TestEvictions(t *testing.T) {

	cache, tbl := createCacheWithCapEntriesInside()

	// now we insert Capacity new entries which should evict all the previously inserted ones
	for i := Capacity; i < 2*Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}

		str := strconv.Itoa(i)
		b, requestError := cache.RetrieveFromCacheOrCompute(request,
			"Request: "+str, "Urequest: "+str)
		assert.Nil(t, requestError)
		assert.NotNil(t, b)
	}

	// now we verify that entries en tbl are not in the cache
	for req := range tbl {
		assert.False(t, cache.has(req))
	}
}

func TestCacheDriver_Has(t *testing.T) {

	cache, tbl := createCacheWithCapEntriesInside()

	for req := range tbl {
		assert.True(t, cache.has(req))
	}
}

func TestLRUOrder(t *testing.T) {
	cache, _ := createCacheWithCapEntriesInside()

	it := cache.NewCacheIt()
	prevTimeStamp := it.GetCurr().timestamp
	for it.Next(); it.HasCurr(); it.Next() {
		curr := it.GetCurr()
		assert.True(t, prevTimeStamp.After(curr.timestamp))
		prevTimeStamp = curr.timestamp
	}
}

func TestCacheDriver_testTTL(t *testing.T) {

	cache := New(Capacity, .4, TTL, toKey, preProcessRequest, callServices)

	request := &RequestEntry{
		N:    10,
		Time: time.Now(),
	}

	str := "10"
	b, requestError := cache.RetrieveFromCacheOrCompute(request,
		"Request: "+str, "Urequest: "+str)
	assert.Nil(t, requestError)
	assert.NotNil(t, b)

	time.Sleep(TTL) // wait for tt expiration

	assert.False(t, cache.has(request))
}

func TestRandomTouches(t *testing.T) {
	cache, tbl := createCacheWithCapEntriesInside()

	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	var response Response
	for i := 0; i < 1e6; i++ {
		i := rand.Intn(N)
		req := requests[i]
		b, requestError := cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
		assert.Nil(t, requestError)

		err := json.Unmarshal(b.([]byte), &response)
		assert.Nil(t, err)

		assert.Equal(t, cache.getMru().postProcessedResponse, b)
	}
}

func TestCompressRandomTouches(t *testing.T) {
	cache, tbl := createCompressedCacheWithCapEntriesInside()

	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	for i := 0; i < 1e6; i++ {
		i := rand.Intn(N)
		req := requests[i]
		response, requestError := cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
		assert.Nil(t, requestError)

		compressedValue := cache.getMru().postProcessedResponse
		decompressedValue, _ := lz4Decompress(compressedValue.([]byte))
		value := &Response{
			Uresponse: &UResponse{Urequest: &URequest{
				Request: &RequestEntry{
					N:        0,
					Time:     time.Time{},
					PutValue: "",
				},
				PutValue: "",
			}},
			Poem: "",
		}
		err := json.Unmarshal(decompressedValue, value)
		assert.Nil(t, err)
		ref := response.(*Response)
		assert.Equal(t, ref.Poem, value.Poem)
		assert.Equal(t, ref.Uresponse.Urequest.Request.N, value.Uresponse.Urequest.Request.N)
		assert.Equal(t, ref.Uresponse.Urequest.Request.PutValue, value.Uresponse.Urequest.Request.PutValue)
		assert.Equal(t, ref.Uresponse.Urequest.PutValue, value.Uresponse.Urequest.PutValue)
	}
}

func TestCacheDriver_CacheState(t *testing.T) {

	cache, tbl := createCacheWithCapEntriesInside()
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	// some random touches
	for i := 0; i < 100; i++ {
		i := rand.Intn(N)
		req := requests[i]
		_, _ = cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
	}

	state, err := cache.GetState()
	assert.Nil(t, err)
	assert.NotNil(t, state)

	fmt.Print(state)
}

func TestCacheDriver_Clean(t *testing.T) {

	cache, tbl := createCacheWithCapEntriesInside()
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	// some random touches
	for i := 0; i < 100; i++ {
		i := rand.Intn(N)
		req := requests[i]
		_, _ = cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
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

	cache, tbl := createCacheWithCapEntriesInside()
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	for i := 0; i < 1000000; i++ {
		req := requests[0]
		_, err := cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
		assert.Nil(t, err)
	}
}

func TestConcurrency(t *testing.T) {

	const ConcurrencyLevel = 20
	const SuperCap = 3037
	const NumRepeatedCalls = 50

	cache := New(SuperCap, .3, 30*time.Second, toKey, preProcessRequest, callServices)

	tbl := make(map[*RequestEntry]bool)
	for i := 0; i < Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}

		str := strconv.Itoa(i)
		_, _ = cache.RetrieveFromCacheOrCompute(request, "Request: "+str, "Urequest: "+str)
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
						b, requestError := cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
						assert.Nil(t, requestError)

						var response Response
						err := json.Unmarshal(b.([]byte), &response)
						assert.Nil(t, err)
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

	cache := NewWithCompression(SuperCap, .3, 30*time.Second, toKey, toBytes, toVal,
		preProcessRequest, callServicesWithCompression)

	tbl := make(map[*RequestEntry]bool)
	for i := 0; i < Capacity; i++ {
		request := &RequestEntry{
			N:    i + 10,
			Time: time.Now(),
		}

		str := strconv.Itoa(i)
		_, _ = cache.RetrieveFromCacheOrCompute(request, "Request: "+str, "Urequest: "+str)
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
						response, requestError := cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
						assert.Nil(t, requestError)

						ref := response.(*Response)
						assert.Equal(t, ref.Uresponse.Urequest.Request.N, request.N)
						assert.Equal(t, ref.Uresponse.Urequest.Request.PutValue, request.PutValue)
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

	cache, tbl := createCacheWithCapEntriesInside()
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
		_, _ = cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
		isMru, err := cache.isKeyMru(req)
		assert.Nil(t, err)
		assert.True(t, isMru)
	}

	err := cache.LazyRemove(lastRequest)
	assert.Nil(t, err)
	assert.False(t, cache.has(lastRequest))
}

func TestCacheDriver_Contains(t *testing.T) {

	cache, tbl := createCacheWithCapEntriesInside()
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	for i := 0; i < N; i++ {
		req := requests[i]
		_, _ = cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
	}

	for i := 0; i < N; i++ {
		req := requests[i]
		ok, err := cache.Contains(req)
		assert.Nil(t, err)
		assert.True(t, ok)
	}
}

func TestCacheDriver_Touch(t *testing.T) {

	cache, tbl := createCacheWithCapEntriesInside()
	N := len(tbl)
	requests := make([]*RequestEntry, 0, N)
	for req := range tbl {
		requests = append(requests, req)
	}

	for i := 0; i < N; i++ {
		req := requests[i]
		_, _ = cache.RetrieveFromCacheOrCompute(req, "Req", "UReq")
	}

	for i := 0; i < N; i++ {
		req := requests[i]
		err := cache.Touch(req)
		assert.Nil(t, err)

		mru, err := cache.isKeyMru(req)

		assert.Nil(t, err)
		assert.True(t, mru)
	}
}

type Adder struct {
	num1, num2 int
}

func ToMapKey(request interface{}) (string, error) {
	adder := request.(Adder)
	return fmt.Sprintf("%d+%d", adder.num1, adder.num2), nil
}

func CallUServices(request, _ interface{}, _ ...interface{}) (interface{}, *RequestError) {
	adder := request.(Adder)
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
	cache := New(Capacity, 0.5, TTL, ToMapKey, nil, CallUServices)
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

//go test -bench=. -benchmem
//goos: darwin
//goarch: amd64
//pkg: github.com/geniussportsgroup/gateway_cache
