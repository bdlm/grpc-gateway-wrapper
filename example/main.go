package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bdlm/log"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"google.golang.org/grpc"

	httppb "github.com/bdlm/grpc-gateway-wrapper/encoding/http"
	log_interceptor "github.com/bdlm/grpc-gateway-wrapper/interceptor/log"
	"github.com/bdlm/grpc-gateway-wrapper/server"
	pb "github.com/bdlm/grpc-gateway-wrapper/example/proto/go/v1"

	// register a protobuf JSON marshaller as the default gRPC encoder.
	_ "github.com/bdlm/grpc-gateway-wrapper/encoding/json"
)

// Cancel is the application context cancel function.
var Cancel context.CancelFunc

// Conf is a struct containing application configuration values.
var Conf serverEnv

// Ctx is the application context.
var Ctx context.Context

// Mux is the grpc-gateay multiplexer.
var Mux *runtime.ServeMux

// Router is the chi router.
var Router *chi.Mux

// serverEnv represents the environment configuration needed for this server.
type serverEnv struct {
	GrpcAddress string `default:"server:50051" split_words:"true"` // GRPC_ADDRESS
	LogLevel    string `default:"info" split_words:"true"`         // LOG_LEVEL
	ServerEnv   string `default:"prod" split_words:"true"`         // SERVER_ENV
}

// - parse configuration values out of environment variables.
// - set the log level.
// - define the log format.
func init() {
	// parse configuration values out of environment variables.
	if err := envconfig.Process("", &Conf); nil != err {
		panic(errors.Wrap(err, "unable to parse environment variables"))
	}

	// set the log level.
	level, err := log.ParseLevel(Conf.LogLevel)
	if nil != err {
		panic(errors.Wrap(err, "could not parse LOG_LEVEL"))
	}
	log.SetLevel(level)
	log.WithField("level", Conf.LogLevel).Info("log level set")

	// define the log format.
	formatter := &log.TextFormatter{}
	if "dev" == Conf.ServerEnv {
		formatter.ForceTTY = true
		log.Debug("TTY formatting enabled")
	}
	log.SetFormatter(formatter)
}

// - create the application context and start a signal handler.
// - init the grpc-gateway multiplexer and add the protobuf-generated handlers.
// - add the grpc-gateway router middleware.
// - init the gRPC server and register it with the protobuf implementation.
// - init the TCP connection handler.
// - start the gRPC and HTTP servers.
// - shutdown when complete.
func main() {
	// create the application context and start a signal handler.
	Ctx, Cancel = context.WithCancel(context.Background())
	go func() {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(
			interrupt,
			syscall.SIGINT,
			syscall.SIGKILL,
			syscall.SIGQUIT,
			syscall.SIGSTOP,
			syscall.SIGTERM,
		)
		sig := <-interrupt
		log.WithField("signal", sig.String()).Info("signal received, shutting down")
		Cancel()
	}()

	// init the grpc-gateway multiplexer.
	Mux = runtime.NewServeMux(
		// expect JSON data by default.
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			EmitDefaults: true, // don't omit properties with default values.
			OrigName:     true, // encode JSON properties as defined in the protobuf (don't convert to CamelCase).
		}),
		// convert form data to JSON.
		runtime.WithMarshalerOption("application/x-www-form-urlencoded", &httppb.Form{JSONPb: runtime.JSONPb{
			EmitDefaults: true, // don't omit properties with default values.
			OrigName:     true, // encode JSON properties as defined in the protobuf (don't convert to CamelCase).
		}}),
		// add all HTTP headers to the gRPC request context.
		runtime.WithIncomingHeaderMatcher(func(headerName string) (string, bool) {
			return headerName, true
		}),
	)

	// add grpc-gateway REST handlers to the multiplexer.
	err := pb.RegisterK8SHandlerFromEndpoint(
		Ctx,
		Mux,
		Conf.GrpcAddress,
		[]grpc.DialOption{grpc.WithInsecure()},
	)
	if nil != err {
		panic(errors.Wrap(err, "unable to register the grpc-gateway multiplexer with the gRPC server"))
	}

	// create a HTTP router that passes all requests to the grpc-gateway handlers.
	Router = chi.NewRouter()
	Router.Use(
		cors.AllowAll().Handler, // CORS
	)
	Router.NotFound(Mux.ServeHTTP)
	Router.MethodNotAllowed(Mux.ServeHTTP)
	Router.With(
		middleware.RedirectSlashes, // redirect requests with trailing path slash
		middleware.DefaultCompress, // GZIP compression
	)

	// logInterceptor is a middleware to log all HTTP requests and gRPC
	// responses.
	logInterceptor := log_interceptor.Interceptor{
		LogStreamRecvMsg: true,
		LogStreamSendMsg: true,
		LogUnaryReqMsg:   true,
	}

	// init the gRPC server and register it with the protobuf implementation.
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			logInterceptor.StreamInterceptor, // automatically log requests
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			logInterceptor.UnaryInterceptor, // automatically log requests
		)),
	)
	pb.RegisterK8SServer(grpcServer, RPC{})

	// init the TCP connection manager.
	tcpServer, err := server.New(Ctx, Router, grpcServer)
	if nil != err {
		panic(errors.Wrap(err, "could not initialize the TCP connection manager"))
	}

	// start the gRPC and HTTP servers.
	log.Info("starting services")
	tcpServer.ListenAndServe()

	// shutdown when complete.
	<-Ctx.Done()
	tcpServer.Shutdown()
	log.Info("shutdown complete")
}
