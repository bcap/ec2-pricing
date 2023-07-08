package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	ec2T "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bcap/humanize"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/bcap/ec2-pricing/aws"
)

func main() {
	start := time.Now()

	var profile string
	var region string
	var priceFile string
	var verbose bool
	flag.StringVar(&profile, "profile", "default", "which aws profile to use")
	flag.StringVar(&region, "region", "", "load prices for this specific region instead of the default region")
	flag.StringVar(&priceFile, "price-file", "", "use a specific price file instead of downloading the latest one")
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.Parse()

	configureLogging(verbose)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitorMemoryUsage(ctx, 10*time.Second)

	awsClient, err := aws.New(ctx, profile, region)
	panicOnErr(err)

	// set region to whatever the aws client loaded (eg used default region in case of empty region)
	region = awsClient.Config.Region

	group, ctx := errgroup.WithContext(ctx)

	var instanceTypes []ec2T.InstanceTypeInfo
	group.Go(func() error {
		log.WithField("region", region).Info("Listing instance types")
		var err error
		instanceTypes, err = awsClient.ListEC2InstanceTypes(ctx)
		return err
	})

	var prices *aws.Prices
	group.Go(func() error {
		var err error

		if priceFile == "" {
			log.WithField("region", region).Info("Loading current prices from AWS")
			prices, err = awsClient.LoadPrices(ctx, time.Now(), region, "AmazonEC2", "USD")
			return err
		}

		log.WithField("file", priceFile).Info("Loading prices from local file")
		var file *os.File
		file, err = os.Open(priceFile)
		if err != nil {
			return err
		}
		prices, err = aws.LoadPricesData(ctx, file)
		return err
	})

	panicOnErr(group.Wait())

	fmt.Println(instanceTypes != nil)
	fmt.Println(prices != nil)

	logMemoryUsage()

	log.WithField("timeTaken", time.Since(start)).Info("done")
}

func configureLogging(verbose bool) {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
	})

	if verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func monitorMemoryUsage(ctx context.Context, every time.Duration) {
	go func() {
		logMemoryUsage()
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logMemoryUsage()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func logMemoryUsage() {
	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)
	log.WithFields(log.Fields{
		"pid":        os.Getpid(),
		"alloc":      humanize.Bytes(int64(memStats.Alloc)),
		"totalAlloc": humanize.Bytes(int64(memStats.TotalAlloc)),
		"gcRuns":     memStats.NumGC,
	}).Debug("Memory Stats")
}
