// Package log contains interceptor/middleware helpers for logging.
package log

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/bdlm/log"
	std "github.com/bdlm/std/logger"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"crypto/sha1"
	"encoding/base64"
)

// Interceptor contains gRPC interceptor middleware methods that logs the
// request as it comes in and the response as it goes out.
type Interceptor struct {
	LogStreamRecvMsg bool // LogStreamRecvMsg if true will log out the contents of each received stream message
	LogStreamSendMsg bool // LogStreamSendMsg if true will log out the contents of each sent stream message
	LogUnaryReqMsg   bool // LogUnaryReqMsg if true will log out the contents of the request message/argument/parameters
}

// UnaryInterceptor is a grpc interceptor middleware that logs out the request
// as it comes in, and the response as it goes out.
func (li *Interceptor) UnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	// Base fields
	fields := map[string]interface{}{
		"gateway-service": path.Dir(info.FullMethod)[1:],
		"gateway-method":  path.Base(info.FullMethod),
	}

	// Request Payload Value
	if li.LogUnaryReqMsg {
		if pb, ok := req.(proto.Message); ok {
			fields["gateway-request"] = pb
		}
	}

	// Add other fields and log the request started
	logRequest(ctx, fields, "request (unary)")

	// Call the handler
	ctx = context.WithValue(ctx, ctxKey{}, fields)
	resp, err := handler(ctx, req)

	// Calculate elapsed time and log the response
	// Re-extract the log fields, as they may have changed
	logResponse(ctx, start, err, "response (unary)")

	// Return the response and error
	return resp, err
}

// StreamInterceptor is a grpc interceptor middleware that logs out the requests
// as they come in and the responses as they go out.
func (li *Interceptor) StreamInterceptor(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()

	// Get the wrapped server stream in order to access any modified context
	// from other interceptors
	wrapped := grpc_middleware.WrapServerStream(stream)
	ctx := wrapped.Context()

	// Base fields
	fields := map[string]interface{}{
		"service": path.Dir(info.FullMethod)[1:],
		"method":  path.Base(info.FullMethod),
	}

	// Grap a log entry with just the base fields, for each streaming
	// send/receive
	streamEntry := log.WithFields(log.Fields(fields))

	// Add other fields and log the request started
	logRequest(ctx, fields, "request (stream)")
	wrapped.WrappedContext = context.WithValue(ctx, ctxKey{}, fields)

	// Call the handler
	err := handler(srv, &loggingServerStream{ServerStream: wrapped, entry: streamEntry, li: li})

	// Calculate elapsed time and log the response
	// Re-extract the log fields, as they may have changed
	logResponse(wrapped.Context(), start, err, "response (stream)")

	// Return the error
	return err
}

// logRequest adds additional log fields for the peer address and metadata,
// and then will log out the request access at info level.
func logRequest(ctx context.Context, fields map[string]interface{}, msg string) {

	// metadata and headers.
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for k, v := range md {
			fields[k] = v
		}

		requestID := ""
		if v, ok := fields["user-agent"]; ok {
			requestID = fmt.Sprintf("%s%s", requestID, v)
		}
		if v, ok := fields["x-forwarded-for"]; ok {
			requestID = fmt.Sprintf("%s%s", requestID, v)
		}
		if "" != requestID {
			hash := sha1.New()
			hash.Write([]byte(requestID))
			fields[":request-id"] = base64.URLEncoding.EncodeToString(hash.Sum(nil))
		}
	}

	// peer address
	if peerAddr, ok := peer.FromContext(ctx); ok {
		address := peerAddr.Addr.String()
		if address != "" &&
			!strings.HasPrefix(address, "127.0.0.1") &&
			!strings.HasPrefix(address, "localhost") {
			// strip the port and any brackets (IPv6)
			address = strings.TrimFunc(
				address[:strings.LastIndexByte(address, byte(':'))],
				func(r rune) bool {
					return '[' == r || ']' == r
				},
			)
			fields["peer"] = address
		}
	}

	log.WithFields(log.Fields(fields)).Info(msg)
}

// marshaller is the marshaller used for serializing protobuf messages.
var marshaller = &jsonpb.Marshaler{
	EmitDefaults: true,
	OrigName: true,
}

// ctxKey is the key to use to lookup the logging fields map in the context.
type ctxKey struct{}

// logResponse calculates the elapsed time and the status code, and then
// will log out the response has finished at an appropriate level.
func logResponse(ctx context.Context, start time.Time, err error, msg string) {
	var fields map[string]interface{}
	var ok bool
	if fields, ok = ctx.Value(ctxKey{}).(map[string]interface{}); !ok {
		fields = map[string]interface{}{}
	}

	// Calculate the elapsed time
	fields["elapsed"] = time.Since(start).Nanoseconds()
	fields["start"] = start.Format(time.RFC3339Nano)

	// Response code
	code := status.Code(err)
	fields["code"] = code

	// Log the response finished
	levelLog(log.WithFields(log.Fields(fields)), DefaultCodeToLevel(code), msg)
}

// jsonpbMarshaler lets a proto interface be marshalled into json
type jsonpbMarshaler struct {
	proto.Message
}

// MarshalJSON lets jsonpbMarshaler implement json interface
func (j *jsonpbMarshaler) MarshalJSON() ([]byte, error) {
	b := &bytes.Buffer{}
	if err := marshaller.Marshal(b, j.Message); err != nil {
		return nil, fmt.Errorf("jsonpb serializer failed: %v", err)
	}
	return b.Bytes(), nil
}

// loggingServerStream wraps a ServerStream in order to log each send and
// receive.
type loggingServerStream struct {
	grpc.ServerStream
	entry *log.Entry
	li    *Interceptor
}

// SendMsg lets loggingServerStream implement ServerStream, and will log sends.
func (l *loggingServerStream) SendMsg(m interface{}) error {
	err := l.ServerStream.SendMsg(m)
	if l.li.LogStreamSendMsg {
		logProtoMessageAsJSON(l.entry, m, status.Code(err), "value", "StreamSend")
	}
	return err
}

// RecvMsg lets loggingServerStream implement ServerStream, and will log
// receives.
func (l *loggingServerStream) RecvMsg(m interface{}) error {
	err := l.ServerStream.RecvMsg(m)
	if l.li.LogStreamRecvMsg {
		logProtoMessageAsJSON(l.entry, m, status.Code(err), "value", "StreamRecv")
	}
	return err
}

// logProtoMessageAsJSON logs an incoming or outgoing protobuf message as JSON.
func logProtoMessageAsJSON(
	entry *log.Entry,
	pbMsg interface{},
	code codes.Code,
	key string,
	msg string,
) {
	if p, ok := pbMsg.(proto.Message); ok {
		levelLog(entry.WithFields(log.Fields{key: &jsonpbMarshaler{p}, "code": code}), DefaultCodeToLevel(code), msg)
	} else {
		levelLog(entry.WithField("code", code), DefaultCodeToLevel(code), msg)
	}
}

// levelLog logs an entry and message at the appropriate levell
func levelLog(entry *log.Entry, level std.Level, msg string) {
	switch level {
	case log.DebugLevel:
		entry.Debug(msg)
	case log.InfoLevel:
		entry.Info(msg)
	case log.WarnLevel:
		entry.Warning(msg)
	case log.ErrorLevel:
		entry.Error(msg)
	case log.FatalLevel:
		entry.Fatal(msg)
	case log.PanicLevel:
		entry.Panic(msg)
	}
}

// DefaultCodeToLevel is the default implementation of gRPC return codes to log
// levels for server side.
func DefaultCodeToLevel(code codes.Code) std.Level {
	switch code {
	case codes.OK:
		return log.InfoLevel
	case codes.Canceled:
		return log.InfoLevel
	case codes.InvalidArgument:
		return log.InfoLevel
	case codes.NotFound:
		return log.InfoLevel
	case codes.AlreadyExists:
		return log.InfoLevel
	case codes.Unauthenticated:
		return log.InfoLevel

	case codes.DeadlineExceeded:
		return log.WarnLevel
	case codes.PermissionDenied:
		return log.WarnLevel
	case codes.ResourceExhausted:
		return log.WarnLevel
	case codes.FailedPrecondition:
		return log.WarnLevel
	case codes.Aborted:
		return log.WarnLevel
	case codes.OutOfRange:
		return log.WarnLevel
	case codes.Unavailable:
		return log.WarnLevel

	case codes.Unknown:
		return log.ErrorLevel
	case codes.Unimplemented:
		return log.ErrorLevel
	case codes.Internal:
		return log.ErrorLevel
	case codes.DataLoss:
		return log.ErrorLevel
	default:
		return log.ErrorLevel
	}
}
