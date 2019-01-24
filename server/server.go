package server

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/bdlm/log"
	"github.com/kelseyhightower/envconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	// gzip encode GRPC responses
	_ "google.golang.org/grpc/encoding/gzip"
)

// - process configuration values out of environment variables
func init() {
	if err := envconfig.Process("", &Conf); nil != err {
		panic(err)
	}
}

// Conf contains the server configuration values.
var Conf serverEnv

// ReadTimeout defines the default server read timeout.
var ReadTimeout = 5 * time.Minute

// WriteTimeout defines the default server write timeout.
var WriteTimeout = 5 * time.Minute

// IdleTimeout defines the default server idle timeout.
var IdleTimeout = 5 * time.Minute

// Server defines metadata for managing gRPC and REST servers.
type Server struct {
	cancel     context.CancelFunc
	ctx        context.Context
	grpcServer *grpc.Server
	httpServer *http.Server
	wg         *sync.WaitGroup
}

// serverEnv defines the environment configuration needed for this server.
type serverEnv struct {
	GrpcAddress string `default:":50051" split_words:"true"` // GRPC_ADDRESS
	RestAddress string `default:":80" split_words:"true"`    // REST_ADDRESS
}

// New returns a new gRPC/REST service handler.
func New(ctx context.Context, handler http.Handler, grpcServer *grpc.Server) (*Server, error) {
	if nil == grpcServer {
		err := errors.New("nil grpcServer value passed")
		log.WithError(err).Error("cannot create service handlers")
		return nil, err
	}

	// create a cancelable server context to handle service shutdown.
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	return &Server{
		ctx:        ctx,
		cancel:     cancel,
		grpcServer: grpcServer,
		httpServer: &http.Server{
			Addr:         Conf.RestAddress,
			Handler:      handler,
			IdleTimeout:  IdleTimeout,
			ReadTimeout:  ReadTimeout,
			WriteTimeout: WriteTimeout,
		},
		wg: &sync.WaitGroup{},
	}, nil
}

// ListenAndServe starts the gRPC and REST gateway services.
func (server *Server) ListenAndServe() {

	// enable service discovery.
	reflection.Register(server.grpcServer)

	// start the gRPC server.
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		log.Info("starting gRPC server")
		listener, err := net.Listen("tcp", Conf.GrpcAddress)
		if nil != err {
			server.cancel()
			panic(errors.Wrap(err, "could not create TCP listener"))
		}
		if err := server.grpcServer.Serve(listener); nil != err {
			server.cancel()
			panic(errors.Wrap(err, "could not start gRPC server"))
		}
	}()

	// start the HTTP server.
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		log.Info("starting HTTP server")
		if err := server.httpServer.ListenAndServe(); nil != err && http.ErrServerClosed != err {
			server.cancel()
			panic(errors.Wrap(err, "could not start HTTP server"))
		}
	}()

	// activate the shutdown handler.
	go func() {
		<-server.ctx.Done()

		// shutdown gRPC server
		go func() {
			log.Info("stopping gRPC server")
			server.grpcServer.GracefulStop()
			log.Info("gRPC shutdown complete")
		}()

		// shutdown HTTP server
		go func() {
			log.Info("stopping HTTP server")
			ctx, cancel := context.WithTimeout(context.Background(), ReadTimeout)
			defer cancel() // don't let context leak; cancel on exit
			if err := server.httpServer.Shutdown(ctx); nil != err {
				log.WithError(err).Warn("Unable to gracefully handle all HTTP connections")
			}
			log.Info("HTTP shutdown complete")
		}()
	}()
}

// Shutdown gracefully shuts down the gRPC and REST services.
func (server *Server) Shutdown() {
	server.cancel()
	server.wg.Wait()
}
