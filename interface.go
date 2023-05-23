package gw_cache

import "encoding/json"

// ProccessorI is the interface that wraps the basic ToMapKey and CallUServices methods.
// ToMapKey is used to convert the input to a string key
// CallUServices is used to call the upstream services
// T represents the input type to get a value
// K represents the valueitself
type ProccessorI[T any, K any] interface {
	ToMapKey(T) (string, error)
	CallUServices(T) (K, *RequestError) //we will leave the preprocess logic for this function
}

type CompressorI[T any] interface {
	BytesToValue([]byte) (T, error)
	ValueToBytes(T) ([]byte, error)
}

type DefaultCompressor struct{}

func (_ *DefaultCompressor) BytesToValue(in []byte) (interface{}, error) {
	var out interface{}
	err := json.Unmarshal(in, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (_ *DefaultCompressor) ValueToBytes(in interface{}) ([]byte, error) {

	return json.Marshal(in)
}
