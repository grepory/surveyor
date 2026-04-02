package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	sdk "surveyor/sdk/go"
	pb "surveyor/sdk/go/gen/probev1"
)

var meta = &pb.ProbeMetadata{
	Id:                "aws-vpc-default-sg",
	Version:           "0.1.0",
	EvidenceType:      "network-boundary-controls",
	SupportsStreaming: true,
}

type SGEvidence struct {
	VPCID           string        `json:"vpc_id"`
	SecurityGroupID string        `json:"security_group_id"`
	Region          string        `json:"region"`
	IngressRules    []IngressRule `json:"ingress_rules"`
	HasPublicAccess bool          `json:"has_public_access"`
}

type IngressRule struct {
	Protocol string   `json:"protocol"`
	FromPort int32    `json:"from_port"`
	ToPort   int32    `json:"to_port"`
	CIDRs    []string `json:"cidrs"`
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Info,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	hostname, _ := os.Hostname()

	probe := &sdk.ProbeFunc{
		Meta: meta,
		OnStream: func(ctx context.Context, req *pb.CollectRequest, send func(*pb.Envelope) error) error {
			regions := extractStringList(req.Config, "regions")
			if len(regions) == 0 {
				regions = []string{"us-east-1"}
			}
			roleARN := extractString(req.Config, "role_arn")
			timeoutSec := extractFloat(req.Config, "timeout_seconds")
			if timeoutSec == 0 {
				timeoutSec = 30
			}

			for _, region := range regions {
				regionCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
				err := collectRegion(regionCtx, region, roleARN, hostname, req.CollectionTrigger, send)
				cancel()
				if err != nil {
					errEnv := sdk.NewErrorEnvelope(meta, hostname, "", runtime.GOOS, req.CollectionTrigger,
						fmt.Errorf("region %s: %w", region, err))
					if sendErr := send(errEnv); sendErr != nil {
						return sendErr
					}
				}
			}
			return nil
		},
	}

	sdk.Serve(probe, logger)
}

func collectRegion(ctx context.Context, region, roleARN, hostname, trigger string, send func(*pb.Envelope) error) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	if roleARN != "" {
		stsClient := sts.NewFromConfig(cfg)
		creds := stscreds.NewAssumeRoleProvider(stsClient, roleARN)
		cfg.Credentials = aws.NewCredentialsCache(creds)
	}

	ec2Client := ec2.NewFromConfig(cfg)

	vpcOut, err := ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return fmt.Errorf("describing VPCs: %w", err)
	}

	for _, vpc := range vpcOut.Vpcs {
		vpcID := aws.ToString(vpc.VpcId)

		sgOut, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{vpcID}},
				{Name: aws.String("group-name"), Values: []string{"default"}},
			},
		})
		if err != nil {
			errEnv := sdk.NewErrorEnvelope(meta, hostname, "", runtime.GOOS, trigger,
				fmt.Errorf("vpc %s in %s: describing security groups: %w", vpcID, region, err))
			if sendErr := send(errEnv); sendErr != nil {
				return sendErr
			}
			continue
		}

		for _, sg := range sgOut.SecurityGroups {
			evidence := SGEvidence{
				VPCID:           vpcID,
				SecurityGroupID: aws.ToString(sg.GroupId),
				Region:          region,
			}

			for _, rule := range sg.IpPermissions {
				ir := IngressRule{
					Protocol: aws.ToString(rule.IpProtocol),
				}
				if rule.FromPort != nil {
					ir.FromPort = *rule.FromPort
				}
				if rule.ToPort != nil {
					ir.ToPort = *rule.ToPort
				}
				for _, cidr := range rule.IpRanges {
					ir.CIDRs = append(ir.CIDRs, aws.ToString(cidr.CidrIp))
					if aws.ToString(cidr.CidrIp) == "0.0.0.0/0" {
						evidence.HasPublicAccess = true
					}
				}
				for _, cidr := range rule.Ipv6Ranges {
					ir.CIDRs = append(ir.CIDRs, aws.ToString(cidr.CidrIpv6))
					if aws.ToString(cidr.CidrIpv6) == "::/0" {
						evidence.HasPublicAccess = true
					}
				}
				evidence.IngressRules = append(evidence.IngressRules, ir)
			}

			payload, err := json.Marshal(evidence)
			if err != nil {
				return fmt.Errorf("marshaling evidence for vpc %s: %w", vpcID, err)
			}

			env := &pb.Envelope{
				Probe:             meta,
				Host:              hostname,
				Platform:          runtime.GOOS,
				CollectedAt:       timestamppb.New(time.Now()),
				ContentType:       "application/json",
				Payload:           payload,
				CollectionTrigger: trigger,
				Status:            pb.CollectStatus_COLLECT_STATUS_SUCCESS,
			}
			if err := send(env); err != nil {
				return err
			}
		}
	}
	return nil
}

// Helper functions to extract config values from protobuf Struct.

func extractStringList(s *structpb.Struct, key string) []string {
	if s == nil {
		return nil
	}
	v, ok := s.Fields[key]
	if !ok {
		return nil
	}
	list := v.GetListValue()
	if list == nil {
		return nil
	}
	var result []string
	for _, item := range list.Values {
		result = append(result, item.GetStringValue())
	}
	return result
}

func extractString(s *structpb.Struct, key string) string {
	if s == nil {
		return ""
	}
	v, ok := s.Fields[key]
	if !ok {
		return ""
	}
	return v.GetStringValue()
}

func extractFloat(s *structpb.Struct, key string) float64 {
	if s == nil {
		return 0
	}
	v, ok := s.Fields[key]
	if !ok {
		return 0
	}
	return v.GetNumberValue()
}
