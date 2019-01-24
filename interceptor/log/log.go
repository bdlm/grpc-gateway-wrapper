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
)

// JSONPbMarshaller is the marshaller used for serializing protobuf messages.
var JSONPbMarshaller = &jsonpb.Marshaler{EmitDefaults: false, OrigName: true}

// ctxKeyFields is the key to use to lookup the logging fields map
type ctxKeyFields struct{}

// GetFields returns the log fields set so far.
func GetFields(ctx context.Context) map[string]interface{} {
	if ctx != nil {
		if fields, ok := ctx.Value(ctxKeyFields{}).(map[string]interface{}); ok {
			return fields
		}
	}
	return map[string]interface{}{}
}

// Interceptor contains gRPC interceptor middleware methods that logs the
// request as it comes in and the response as it goes out.
type Interceptor struct {
	LogUnaryReqMsg   bool // LogUnaryReqMsg if true will log out the contents of the request message/argument/parameters
	LogStreamRecvMsg bool // LogStreamRecvMsg if true will log out the contents of each received stream message
	LogStreamSendMsg bool // LogStreamSendMsg if true will log out the contents of each sent stream message
}

// UnaryInterceptor is a grpc interceptor middleware that logs out the request
// as it comes in, and the response as it goes out.
func (li *Interceptor) UnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {

	// Base fields
	fields := map[string]interface{}{
		"service": path.Dir(info.FullMethod)[1:],
		"method":  path.Base(info.FullMethod),
	}

	// Request Payload Value
	if li.LogUnaryReqMsg {
		if pb, ok := req.(proto.Message); ok {
			fields["value"] = &jsonpbMarshalleble{pb}
		}
	}

	// Add other fields and log the request started
	addFieldsAndLogRequest(ctx, fields, "request (unary)")

	// Call the handler
	start := time.Now()
	ctx = context.WithValue(ctx, ctxKeyFields{}, fields)
	resp, err := handler(ctx, req)

	// Calculate elapsed time and log the response
	// Re-extract the log fields, as they may have changed
	addFieldsAndLogResponse(GetFields(ctx), start, err, "response (unary)")

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
	addFieldsAndLogRequest(ctx, fields, "request (stream)")
	wrapped.WrappedContext = context.WithValue(ctx, ctxKeyFields{}, fields)

	// Call the handler
	start := time.Now()
	err := handler(srv, &loggingServerStream{ServerStream: wrapped, entry: streamEntry, li: li})

	// Calculate elapsed time and log the response
	// Re-extract the log fields, as they may have changed
	addFieldsAndLogResponse(GetFields(wrapped.Context()), start, err, "response (stream)")

	// Return the error
	return err
}

// addFieldsAndLogRequest adds additional log fields for the peer address and
// metadata, and then will log out the request access at info level.
func addFieldsAndLogRequest(ctx context.Context, fields map[string]interface{}, msg string) {

	// metadata and headers.
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ua := md["grpcgateway-user-agent"]; len(ua) != 0 {
			fields["ua"] = ua[0]
		} else if ua = md["user-agent"]; len(ua) != 0 {
			fields["ua"] = ua[0]
		}
		if ref := md["grpcgateway-referer"]; len(ref) != 0 {
			fields["referer"] = ref[0]
		} else if ref = md["referer"]; len(ref) != 0 {
			fields["referer"] = ref[0]
		}
		if host := md["x-forwarded-host"]; len(host) != 0 {
			fields["host"] = host[0]
		}
		if ip := md["x-forwarded-for"]; len(ip) != 0 {
			fields["ip"] = ip[0]
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

// addFieldsAndLogResponse calculates the elapsed time and the status code,
// and then will log out the response has finished at an appropriate level.
func addFieldsAndLogResponse(fields map[string]interface{}, start time.Time, err error, msg string) {

	// Calculate the elapsed time
	fields["elapsed"] = time.Since(start).Nanoseconds()
	fields["start"] = start.Format(time.RFC3339Nano)

	// Response code
	code := status.Code(err)
	fields["code"] = code

	// Log the response finished
	levelLog(log.WithFields(log.Fields(fields)), DefaultCodeToLevel(code), msg)
}

// jsonpbMarshalleble lets a proto interface be marshalled into json
type jsonpbMarshalleble struct {
	proto.Message
}

// MarshalJSON lets jsonpbMarshalleble implement json interface
func (j *jsonpbMarshalleble) MarshalJSON() ([]byte, error) {
	b := &bytes.Buffer{}
	if err := JSONPbMarshaller.Marshal(b, j.Message); err != nil {
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
		levelLog(entry.WithFields(log.Fields{key: &jsonpbMarshalleble{p}, "code": code}), DefaultCodeToLevel(code), msg)
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
	case codes.Unknown:
		return log.ErrorLevel
	case codes.InvalidArgument:
		return log.InfoLevel
	case codes.DeadlineExceeded:
		return log.WarnLevel
	case codes.NotFound:
		return log.InfoLevel
	case codes.AlreadyExists:
		return log.InfoLevel
	case codes.PermissionDenied:
		return log.WarnLevel
	case codes.Unauthenticated:
		return log.InfoLevel // unauthenticated requests can happen
	case codes.ResourceExhausted:
		return log.WarnLevel
	case codes.FailedPrecondition:
		return log.WarnLevel
	case codes.Aborted:
		return log.WarnLevel
	case codes.OutOfRange:
		return log.WarnLevel
	case codes.Unimplemented:
		return log.ErrorLevel
	case codes.Internal:
		return log.ErrorLevel
	case codes.Unavailable:
		return log.WarnLevel
	case codes.DataLoss:
		return log.ErrorLevel
	default:
		return log.ErrorLevel
	}
}

// DefaultClientCodeToLevel is the default implementation of gRPC return codes
// to log levels for client side.
func DefaultClientCodeToLevel(code codes.Code) std.Level {
	switch code {
	case codes.OK:
		return log.DebugLevel
	case codes.Canceled:
		return log.DebugLevel
	case codes.Unknown:
		return log.InfoLevel
	case codes.InvalidArgument:
		return log.DebugLevel
	case codes.DeadlineExceeded:
		return log.InfoLevel
	case codes.NotFound:
		return log.DebugLevel
	case codes.AlreadyExists:
		return log.DebugLevel
	case codes.PermissionDenied:
		return log.InfoLevel
	case codes.Unauthenticated:
		return log.InfoLevel // unauthenticated requests can happen
	case codes.ResourceExhausted:
		return log.DebugLevel
	case codes.FailedPrecondition:
		return log.DebugLevel
	case codes.Aborted:
		return log.DebugLevel
	case codes.OutOfRange:
		return log.DebugLevel
	case codes.Unimplemented:
		return log.WarnLevel
	case codes.Internal:
		return log.WarnLevel
	case codes.Unavailable:
		return log.WarnLevel
	case codes.DataLoss:
		return log.WarnLevel
	default:
		return log.InfoLevel
	}
}
