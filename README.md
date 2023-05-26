# A cache for API gateways

- [A cache for API gateways](#a-cache-for-api-gateways)
	- [Installation](#installation)
	- [Declaration](#declaration)
	- [Usage](#usage)
	- [Notes on its implementation](#notes-on-its-implementation)
		- [Parameters selection](#parameters-selection)
		- [Liveness danger](#liveness-danger)
		- [Possible performance issue with internal hash Table](#possible-performance-issue-with-internal-hash-table)

gateway_cache implements a simple cache from HTTP requests to their responses. An essential quality is that the cache can receive repeated requests before computing the first one. In such a case, the repeated requests will wait without contention for other different requests until the response is ready.

Once the response is ready, the runtime will free the retained repeated requests, and the flow will continue in a usual way.

## Installation

```Bash
 go get -U github.com/geniussportsgroup/gateway_cache
```

## Declaration

The user can use the cache from any GO HTTP middleware.  

To use it, declare something like that:  

```Go 
var cache *Cache.CacheDriver = Cache.New[T,K](cacheCapacity, capFactor, cacheTTL,mapper)
```

* `T`: This is the request type. This type will be receive to execute the request.
* `K`: The response type. This type will be the type that the request will response.
*   `cacheCapacity`: The maximum number of entries that the cache can store. Once this limit reaches and one wants to insert a new entry, the runtime selects the LRU entry for eviction.
*   `capFactor`: A multiplicative factor for increasing the physical size of the internal hash table. Using a physical size larger than the logical size reduces the possibility of an undesired table resizing when a new request arrives.
*   `cacheTTL`: A soft duration time for the cache entry. By soft, we mean that the duration does not avoid eviction.
*   `processor`: Is an interface with this contract

```Go
type ProcessorI[T, K any] interface {
	ToMapKey(T) (string, error)
	CacheMissSolver(T) (K, *models.RequestError) 
}
``` 

* `T and K`: Are the same defined in cache's creation 

* `ToMapKey` is a function used for transforming the request to a string. This function is used for mapping request to cache entries, which at the end contain the response.
*   `CacheMissSolver`: this is the core function in charge of calling the microservices, gathering the request, assembly them in an HTPP response, and eventually compressing it. The result is store in the cache entry.

## Usage

In your request handler, you should put a line like this one:  

```Go
    gzipResponse, predictError := cache.RetrieveFromCacheOrCompute(request)  
```

The first result is the request itself already prepared by the `CacheMissSolver` function. The second result is an error indication. If its value is not `nil`, then the first parameter is `nil`.  

If it is the first request, then the flow blocks, but the process coded in `CacheMissSolver` is triggered. The following repeated requests before to get the result block too, but they do not cause contention on other requests that are not related to the original one.  

Once the result is gotten (by `CacheMissSolver` function), all the retained requests are unblocked, and the flow continues as usual.  

If the result is already in the cache, then the cache retrieves the result, return it, and the flow continues in a usual way without blocking.

## Notes on its implementation  

### Parameters selection

The most important thing to know is the maximum number of simultaneous request that a gateway could receive. Once this number is known, the cache recommended cache capacity should be at least 30 % larger. The larger the capacity is, the more performant the cache should be.

### Liveness danger

The cache could start to reject requests if it receives more different requests in a short time than its capacity.

### Possible performance issue with internal hash Table

Internally, the cache handles a GO map from a string version of a request to a cache entry containing control information and the request and its response. The GO maps are implemented through a hash table that eventually could require a resize when the load factor becomes high (about 80 % of occupation). A new request arrival could cause, since this the only moment when caching insertions occur. Consequently, this could slow down the response time. The proper way for avoiding this event is to ensure that the table size is always enough. That implies that the size should always be greater than the number of inserted entries. This is the reason for the capFactor parameter received by the constructor.

We advise using a capFactor of at least 0.5 or more.
