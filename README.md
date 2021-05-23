# A cache for API gateways

gateway\_cache implements a simple cache from HTTP request to their responses. An essential quality is that the cache can receive repeated requests before the first one has been computed. I such a case, the repeated request will wait without contention for other different requests until the response is ready.

  

Once the response is ready, the runtime will free the retained repeated requests, and the flow will continue in a usual way.

## Declaration

  

### Usage

  

### Notes on its implementation

  

Parameters selection

  

Liveness danger

  

Hash Table
