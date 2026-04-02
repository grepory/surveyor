package sdk

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	pb "surveyor/sdk/go/gen/probev1"
)

func Serve(impl pb.ProbeServiceServer, logger hclog.Logger) {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Info,
			JSONFormat: true,
		})
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"probe": &ProbeGRPCPlugin{Impl: impl},
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
