package main

import (
	"fmt"
	"sync"
	"time"

	gw_cache "github.com/geniussportsgroup/gateway_cache/v3"
)

type Processor struct {
}

// receive the key defined as int and return a string
func (p *Processor) ToMapKey(someValue int) (string, error) {
	return fmt.Sprint(someValue), nil
}

// receive the value that will be used as a key and return a string, that will be used as a value
func (p *Processor) CacheMissSolver(someValue int, _ ...interface{}) (string, *gw_cache.RequestError) {
	time.Sleep(time.Second * 1)
	return fmt.Sprintf("%d processed", someValue), nil
}

func main() {
	//create the cache
	capacity := 10
	capFactor := 0.6
	ttl := time.Minute * 5
	p := &Processor{}

	cache := gw_cache.New[int, string](
		capacity,
		capFactor,
		ttl,
		ttl,
		p.CacheMissSolver,
		p.ToMapKey,
	)

	// compute and set the value
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			wg.Add(1)
			go func(i int, wg *sync.WaitGroup) {
				start := time.Now()
				value, err := cache.RetrieveFromCacheOrCompute(i)
				fmt.Printf("with i:%d v:%s, e:%v\n elapsed time %s\n", i, value, err, time.Since(start).Abs())
				wg.Done()
			}(i, &wg)
		}
	}
	wg.Wait()
	fmt.Printf("cache.MissCount(): %v\n", cache.MissCount())
	fmt.Printf("cache.HitCount(): %v\n", cache.HitCount())
}
