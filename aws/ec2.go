package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	log "github.com/sirupsen/logrus"
)

func (aws AWS) ListEC2InstanceTypes(ctx context.Context) ([]types.InstanceTypeInfo, error) {
	start := time.Now()
	results := []types.InstanceTypeInfo{}
	var maxResults int32 = 100
	err := paginate(ctx, func(ctx context.Context, nextToken *string) (*string, error) {
		out, err := aws.EC2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
			MaxResults: &maxResults,
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, out.InstanceTypes...)
		log.WithField("instanceTypes", len(results)).Debug("AWS.ListEC2InstanceTypes pagination")
		return out.NextToken, nil
	})

	log.
		WithField("instanceTypes", len(results)).
		WithField("timeTaken", time.Since(start)).
		Debug("AWS.ListEC2InstanceTypes return")
	return results, err
}
