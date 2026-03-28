package api

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

const CodecName = "json"

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Name() string {
	return CodecName
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}
