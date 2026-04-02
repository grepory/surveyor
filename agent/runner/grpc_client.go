package runner

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	sdk "surveyor/sdk/go"
	pb "surveyor/sdk/go/gen/probev1"
)

type ProbeGRPCClient struct {
	plugin.Plugin
}

func (p *ProbeGRPCClient) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}

func (p *ProbeGRPCClient) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pb.NewProbeServiceClient(c), nil
}

func clientPluginMap() map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		"probe": &ProbeGRPCClient{},
	}
}

var handshake = sdk.Handshake
