package sdk

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pb "surveyor/sdk/go/gen/probev1"
)

var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "SURVEYOR_PROBE",
	MagicCookieValue: "surveyor-v1",
}

type ProbeGRPCPlugin struct {
	plugin.Plugin
	Impl pb.ProbeServiceServer
}

func (p *ProbeGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterProbeServiceServer(s, p.Impl)
	return nil
}

func (p *ProbeGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pb.NewProbeServiceClient(c), nil
}

var PluginMap = map[string]plugin.Plugin{
	"probe": &ProbeGRPCPlugin{},
}
