package sdk

import (
	"context"
	"fmt"

	pb "surveyor/sdk/go/gen/probev1"
)

type CollectFunc func(ctx context.Context, req *pb.CollectRequest) (*pb.Envelope, error)
type StreamFunc func(ctx context.Context, req *pb.CollectRequest, send func(*pb.Envelope) error) error

type ProbeFunc struct {
	pb.UnimplementedProbeServiceServer
	Meta      *pb.ProbeMetadata
	OnCollect CollectFunc
	OnStream  StreamFunc
}

func (p *ProbeFunc) Metadata(ctx context.Context, req *pb.MetadataRequest) (*pb.ProbeMetadata, error) {
	return p.Meta, nil
}

func (p *ProbeFunc) Collect(ctx context.Context, req *pb.CollectRequest) (*pb.CollectResponse, error) {
	if p.OnCollect == nil {
		return nil, fmt.Errorf("probe %s does not support Collect", p.Meta.Id)
	}
	env, err := p.OnCollect(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.CollectResponse{Envelope: env}, nil
}

func (p *ProbeFunc) CollectStream(req *pb.CollectRequest, stream pb.ProbeService_CollectStreamServer) error {
	if p.OnStream == nil {
		return fmt.Errorf("probe %s does not support CollectStream", p.Meta.Id)
	}
	return p.OnStream(stream.Context(), req, func(env *pb.Envelope) error {
		return stream.Send(&pb.CollectResponse{Envelope: env})
	})
}
