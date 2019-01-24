package main

import (
	pb "github.com/bdlm/grpc-gateway-wrapper/example/proto/go/v1"
	"golang.org/x/net/context"
)

// RPC defines the protobuf service implementation.
type RPC struct{}

// LivenessProbe returns success.
func (r RPC) LivenessProbe(ctx context.Context, msg *pb.NilMsg) (*pb.ProbeResult, error) {
	result := &pb.ProbeResult{}
	return result, nil
}

// ReadinessProbe returns success.
func (r RPC) ReadinessProbe(ctx context.Context, msg *pb.NilMsg) (*pb.ProbeResult, error) {
	result := &pb.ProbeResult{}
	return result, nil
}
