package gw_cache

import (
	"encoding/json"
)

// ProccessorI is the interface that wraps the basic ToMapKey and CallUServices methods.
// ToMapKey is used to convert the input to a string key
// CallUServices is used to call the upstream services
// T represents the input type to get a value
// K represents the valueitself
//
//go:generate mockery --name ProccessorI --with-expecter=true --filename=proccessor_mock.go --inpackage
type ProccessorI[T, K any] interface {
	ToMapKey(T) (string, error)
	CallUServices(T) (K, *RequestError) //we will leave the preprocess logic for this function

}

// CompressorI is the interface that wraps the basic Compress and Decompress methods.
// Compress is used to compress the input
// Decompress is used to decompress the input
//
//go:generate mockery --name CompressorI --with-expecter=true --filename=compressor_mock.go --inpackage
type CompressorI interface {
	Compress([]byte) ([]byte, error)
	Decompress([]byte) ([]byte, error)
}

// TransformerI is the interface that wraps the basic BytesToValue and ValueToBytes methods.
// BytesToValue is used to convert the input to a value
// ValueToBytes is used to convert the value to a byte array
// T represents the value type
//
//go:generate mockery --name TransformerI --with-expecter=true --filename=transformer_mock.go --inpackage
type TransformerI[T any] interface {
	BytesToValue([]byte) (T, error)
	ValueToBytes(any) ([]byte, error)
}
type DefaultTransformer[T any] struct{}

func (_ *DefaultTransformer[T]) BytesToValue(in []byte) (T, error) {
	var out T
	err := json.Unmarshal(in, &out)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (_ *DefaultTransformer[T]) ValueToBytes(in any) ([]byte, error) {

	return json.Marshal(in)
}
