// Package jsonpb defines a protobuf JSON marshaller and registers it as the
// default gRPC encoder.
package jsonpb

import (
	"bytes"
	"encoding/json"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/encoding"
)

func init() {
	Register(defaultOpts)
}

var defaultOpts = jsonpb.Marshaler{
	EmitDefaults: true,
	OrigName:     true,
}

// Register provides a way to override the jsonpb.Marshaler default values.
// This is not thread-safe outside of init() routines.
func Register(opts jsonpb.Marshaler) {
	encoding.RegisterCodec(jsonMarshaler{
		Marshaler: opts,
	})
}

// jsonMarshaler implements the jsonpb methods.
type jsonMarshaler struct {
	jsonpb.Marshaler
	jsonpb.Unmarshaler
}

// Name returns the codec name.
func (jsonMarshaler) Name() string {
	return "json"
}

// Marshal marshals JSON.
func (j jsonMarshaler) Marshal(v interface{}) (out []byte, err error) {
	if pm, ok := v.(proto.Message); ok {
		b := new(bytes.Buffer)
		err := j.Marshaler.Marshal(b, pm)
		if err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	return json.Marshal(v)
}

// Unmarshal unmarshals JSON.
func (j jsonMarshaler) Unmarshal(data []byte, v interface{}) (err error) {
	if pm, ok := v.(proto.Message); ok {
		b := bytes.NewBuffer(data)
		return j.Unmarshaler.Unmarshal(b, pm)
	}
	return json.Unmarshal(data, v)
}
