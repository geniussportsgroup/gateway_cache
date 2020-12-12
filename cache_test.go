package gw_cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"strconv"
	"testing"
	"time"
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

type UResponse struct {
	Urequest *URequest
}

type Response struct {
	Uresponse *UResponse
}

func toKey(entry interface{}) (string, error) {
	b, err := json.Marshal(*entry.(*RequestEntry))
	if err != nil {
		return "", err
	}
	return string(b), nil
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

func TestNew(t *testing.T) {

	cache := New(100, time.Minute, toKey, preProcessRequest, callServices)

	assert.Equal(t, 100, cache.capacity)
	assert.Equal(t, time.Minute, cache.ttl)
	assert.Equal(t, 0, cache.hitCount)
	assert.Equal(t, 0, cache.missCount)
	assert.Equal(t, 0, cache.numEntries)
}

const Capacity = 31
const TTL = 15 * time.Second

func createCacheWithCapEntriesInside() (*CacheDriver, map[*RequestEntry]bool) {

	cache := New(Capacity, TTL, toKey, preProcessRequest, callServices)

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

func TestCacheProcessing(t *testing.T) {

	cache := New(Capacity, TTL, toKey, preProcessRequest, callServices)

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

	cache := New(Capacity, TTL, toKey, preProcessRequest, callServices)

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
