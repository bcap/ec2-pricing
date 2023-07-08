package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	log "github.com/sirupsen/logrus"
)

func (aws AWS) LoadPrices(
	ctx context.Context, when time.Time, region string,
	serviceCode string, currency string,
) (*Prices, error) {
	listing, err := aws.PriceListing(ctx, when, region, serviceCode, currency)
	if err != nil {
		return nil, err
	}
	arn := listing[0].PriceListArn
	url, err := aws.PriceListURL(ctx, *arn)
	if err != nil {
		return nil, err
	}
	return FetchPrices(ctx, url)
}

func (aws AWS) PriceListing(
	ctx context.Context, when time.Time, region string,
	serviceCode string, currency string,
) ([]types.PriceList, error) {
	start := time.Now()
	results := []types.PriceList{}
	var maxResults int32 = 100
	err := paginate(ctx, func(ctx context.Context, nextToken *string) (*string, error) {
		out, err := aws.Pricing.ListPriceLists(ctx, &pricing.ListPriceListsInput{
			CurrencyCode:  &currency,
			EffectiveDate: &when,
			ServiceCode:   &serviceCode,
			MaxResults:    &maxResults,
			RegionCode:    &region,
			NextToken:     nextToken,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, out.PriceLists...)
		log.WithField("priceLists", len(results)).Debug("AWS.PriceListing pagination")
		return out.NextToken, nil
	})
	log.
		WithField("priceLists", len(results)).
		WithField("timeTaken", time.Since(start)).
		Debug("AWS.PriceListing return")
	return results, err
}

func (aws AWS) PriceListURL(ctx context.Context, priceListARN string) (string, error) {
	start := time.Now()
	format := "json"
	out, err := aws.Pricing.GetPriceListFileUrl(ctx, &pricing.GetPriceListFileUrlInput{
		FileFormat:   &format,
		PriceListArn: &priceListARN,
	})
	if err != nil {
		return "", err
	}
	url := ""
	if out.Url != nil {
		url = *out.Url
	}
	log.
		WithField("url", url).
		WithField("timeTaken", time.Since(start)).
		Debug("AWS.PriceListURL return")
	return url, nil
}

func FetchPrices(ctx context.Context, url string) (*Prices, error) {
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 4 || resp.StatusCode/100 == 5 {
		return nil, fmt.Errorf("got status code %d", resp.StatusCode)
	}
	log.WithField("size", resp.ContentLength).Debug("AWS.FetchPrices decoding body")
	result, err := LoadPricesData(ctx, resp.Body)
	if err != nil {
		return nil, err
	}
	log.
		WithField("size", resp.ContentLength).
		WithField("timeTaken", time.Since(start)).
		Debug("AWS.FetchPrices return")
	return result, err
}

func LoadPricesData(ctx context.Context, reader io.Reader) (*Prices, error) {
	start := time.Now()
	var result Prices
	if err := json.NewDecoder(reader).Decode(&result); err != nil {
		return nil, err
	}
	processPrices(&result)
	log.
		WithField("timeTaken", time.Since(start)).
		Debug("AWS.LoadPricesData return")
	return &result, nil
}
