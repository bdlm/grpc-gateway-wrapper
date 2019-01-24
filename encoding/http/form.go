package http

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/utilities"
)

// formMarshaler is a Marshaler which marshals from Form-Data
// (application/x-www-form-urlencoded), and marshals into JSON
// (application/json), using "github.com/golang/protobuf/jsonpb". It
// supports full protobuf functionality.
//
// It can be added before the MIMEWildcard with:
// `runtime.WithMarshalerOption("application/x-www-form-urlencoded", &runtime.Form{}),`
type Form struct {
	runtime.JSONPb
}

// Confirm *Form is a runtime.Marshaler
var _ runtime.Marshaler = &Form{}

// Unmarshal unmarshals Form "data" into "v"
func (j *Form) Unmarshal(data []byte, v interface{}) error {
	return decodeForm(bytes.NewBuffer(data), v)
}

// NewDecoder returns a Decoder which reads Form data from "r".
func (j *Form) NewDecoder(r io.Reader) runtime.Decoder {
	return runtime.DecoderFunc(func(v interface{}) error {
		return decodeForm(r, v)
	})
}

// decodeForm reads and parses form data from "r" by using
// runtime.PopulateQueryParameters, then populates this into "v".
// This method fails if "v" is not a proto.Message.
func decodeForm(d io.Reader, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("not proto message")
	}

	formData, err := ioutil.ReadAll(d)
	if err != nil {
		return err
	}

	values, err := url.ParseQuery(string(formData))
	if err != nil {
		return err
	}

	err = runtime.PopulateQueryParameters(msg, values, &utilities.DoubleArray{})
	if err != nil {
		return err
	}

	return nil
}
