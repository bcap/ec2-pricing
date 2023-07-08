package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
)

type AWS struct {
	Config  *aws.Config
	EC2     *ec2.Client
	Pricing *pricing.Client
}

func New(ctx context.Context, profile string, region string) (AWS, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile), config.WithRegion(region))
	if err != nil {
		return AWS{}, err
	}
	return AWS{
		Config:  &cfg,
		EC2:     ec2.NewFromConfig(cfg),
		Pricing: pricing.NewFromConfig(cfg),
	}, nil
}

const PaginationLimit = 1000

type pageFn = func(ctx context.Context, nextToken *string) (*string, error)

var ErrRepeatedToken = errors.New("pagination function returned repeated token, which would lead to an infinite loop")

func paginate(ctx context.Context, pageFn pageFn) error {
	var token *string
	var err error
	seemTokens := map[string]struct{}{}
	for i := 0; i < PaginationLimit; i++ {
		token, err = pageFn(ctx, token)
		if err != nil {
			return err
		}
		if token == nil || *token == "" {
			return nil
		}
		if _, ok := seemTokens[*token]; ok {
			return ErrRepeatedToken
		}
		seemTokens[*token] = struct{}{}
	}
	return errors.New("too many pages to iterate")
}
