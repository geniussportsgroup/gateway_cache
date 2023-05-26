package main

import (
	"fmt"
	"time"

	gw_cache "github.com/geniussportsgroup/gateway_cache"
	"github.com/geniussportsgroup/gateway_cache/models"
)

type Processor struct {
}

// receive the key defined as int and return a string
func (p *Processor) ToMapKey(someValue int) (string, error) {
	return fmt.Sprint(someValue), nil
}

// receive the value that will be used as a key and return a string, that will be used as a value
func (p *Processor) CacheMissSolver(someValue int) (string, *models.RequestError) {
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
		p,
	)

	// compute and set the value
	value, err := cache.RetrieveFromCacheOrCompute(3)
	fmt.Println(value, err) // 3 processed <nil>
}
