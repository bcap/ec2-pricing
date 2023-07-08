package aws

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

//
// types
//

type Prices struct {
	FormatVersion string             `json:"formatVersion"`
	Disclaimer    string             `json:"disclaimer"`
	Version       string             `json:"version"`
	Publication   time.Time          `json:"publicationDate"`
	OfferCode     string             `json:"offerCode"`
	Products      map[SKU]*Product   `json:"products"`
	Terms         map[TermType]Terms `json:"terms"`
	Attributes    any                `json:"attributeList"`
}

type SKU = string

type Product struct {
	SKU        SKU               `json:"sku"`
	Family     string            `json:"productFamily"`
	Attributes ProductAttributes `json:"attributes"`
}

// docs at https://docs.aws.amazon.com/cur/latest/userguide/product-columns.html
type ProductAttributes struct {
	// Meta / Misc
	Region            string  `json:"regionCode"`
	InstanceFamily    string  `json:"instanceFamily"`
	InstanceType      string  `json:"instanceType"`
	CurrentGeneration bool    `json:"currentGeneration"`
	OperatingSystem   string  `json:"operatingSystem"`
	LicenseModel      string  `json:"licenseModel"`
	Tenancy           string  `json:"tenancy"`
	Capacitystatus    string  `json:"capacitystatus"`
	SizeFactor        float64 `json:"normalizationSizeFactor"`

	// CPU / GPU
	Processor         InstanceProcessor `json:"physicalProcessor"`
	ProcessorFeatures []string          `json:"processorFeatures"`
	ClockSpeed        Range[float64]    `json:"clockSpeed"`
	VCPUs             int               `json:"vcpu"`
	GPUs              int               `json:"gpu"`

	// Memory
	MemoryMiB    int64  `json:"memory"`
	GPUMemoryMiB string `json:"gpuMemory"`

	// IO
	Storage                    InstanceStorage    `json:"storage"`
	NetworkPerformanceGbps     NetworkPerformance `json:"networkPerformance"`
	DedicatedEBSThroughputGbps NetworkPerformance `json:"dedicatedEbsThroughput"`

	// Features
	EnhancedNetworkingSupported bool `json:"enhancedNetworkingSupported"`
}

type InstanceStorage struct {
	Amount int
	SizeMB int
	Type   string
}

type InstanceProcessor struct {
	Make  string
	Model string
}

type Range[T any] struct {
	Unit string
	Min  T
	Max  T
}

type NetworkPerformance struct {
	Range[float64]
	Description string
}

type TermType = string

const TermTypeOnDemand TermType = "OnDemand"
const TermTypeReserved TermType = "Reserved"

type Terms = map[SKU]map[string]*Term

type Term struct {
	SKU             SKU                       `json:"sku"`
	OfferTermCode   string                    `json:"offerTermCode"`
	EffectiveDate   time.Time                 `json:"effectiveDate"`
	PriceDimensions map[string]PriceDimension `json:"priceDimensions"`
	Attributes      map[string]any            `json:"termAttributes"`
	Product         *Product
}

type PriceDimension struct {
	RateCode     string            `json:"rateCode"`
	Description  string            `json:"description"`
	BeginRange   string            `json:"beginRange"`
	EndRange     string            `json:"endRange"`
	Unit         string            `json:"unit"`
	PricePerUnit map[string]string `json:"pricePerUnit"`
}

//
// encoding/json.Unmarshaller implementations
//

func (a *ProductAttributes) UnmarshalJSON(data []byte) error {
	type Raw struct {
		ProductAttributes
		CurrentGeneration           string   `json:"currentGeneration"`
		SizeFactor                  string   `json:"normalizationSizeFactor"`
		ProcessorFeatures           string   `json:"processorFeatures"`
		VCPUs                       string   `json:"vcpu"`
		GPUs                        string   `json:"gpu"`
		ClockSpeed                  string   `json:"clockSpeed"`
		MemoryMib                   string   `json:"memory"`
		NetworkPerformance          string   `json:"networkPerformance"`
		EnhancedNetworkingSupported string   `json:"enhancedNetworkingSupported"`
		UnmarshalJSON               struct{} // avoid recursion loop
	}

	// // DEBUG - START
	// m := map[string]any{}
	// json.Unmarshal(data, &m)
	// if m["instanceType"] == "r6idn.24xlarge" {
	// 	fmt.Println()
	// }
	// // DEBUG - END

	var raw Raw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*a = ProductAttributes(raw.ProductAttributes)
	a.CurrentGeneration = ParseBool(raw.CurrentGeneration)
	a.SizeFactor = ParseFloat(raw.SizeFactor, -1)
	a.ProcessorFeatures = strings.Split(raw.ProcessorFeatures, "; ")
	a.VCPUs = int(ParseInt(raw.VCPUs, -1))
	a.GPUs = int(ParseInt(raw.GPUs, 0))
	a.ClockSpeed = ParseClockSpeed(raw.ClockSpeed)
	a.MemoryMiB = ParseMemoryMib(raw.MemoryMib)
	a.EnhancedNetworkingSupported = ParseBool(raw.EnhancedNetworkingSupported)

	return nil
}

func (p *InstanceProcessor) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	if str == "" {
		return nil
	}

	*p = InstanceProcessor{Model: str}

	recognizedMakes := []string{"Intel", "AMD", "AWS"}

	for _, recognizedMake := range recognizedMakes {
		idx := strings.Index(str, recognizedMake)
		if idx == -1 {
			continue
		}
		p.Make = recognizedMake
		if idx == 0 {
			p.Model = strings.TrimSpace(str[len(recognizedMake):])
		} else {
			p.Model = str
		}
		break
	}
	return nil
}

// Example strings & matches:
// - "2 x 1900 NVMe SSD"    -> multiplier: 2, size: 1900, unit: ,   type: NVMe SSD
// - "4 x 2000 HDD"         -> multiplier: 4, size: 2000, unit: ,   type: HDD
// - "225 GB NVMe SSD"      -> multiplier: ,  size: 225,  unit: GB, type: NVMe SSD
// - "2 x 3800 GB NVMe SSD" -> multiplier: 2, size: 3800, unit: GB, type: NVMe SSD
var storagePattern = regexp.MustCompile(`` +
	`^` +
	`(?:(\d+) x\s+)?` + // multiplier
	`(\d+)` + // size
	`\s+` +
	`(?:(\w+)\s+)?` + // unit
	`(.+)` + // type/extra
	`$`,
)

func (s *InstanceStorage) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	if str == "" {
		return nil
	}

	if str == "EBS Only" {
		*s = InstanceStorage{Type: str}
		return nil
	}

	match := storagePattern.FindStringSubmatch(str)
	if match == nil {
		*s = InstanceStorage{Type: str}
		return nil
	}

	*s = InstanceStorage{
		Amount: int(ParseInt(match[1], 1)),
		SizeMB: int(ParseInt(match[2], 0)),
		Type:   match[4],
	}
	if strings.ToLower(match[3]) == "gb" {
		s.SizeMB *= 1000
	}
	return nil
}

func (p *NetworkPerformance) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	if str == "" {
		return nil
	}

	str = strings.ToLower(str)

	// based on this SO comment:  https://stackoverflow.com/questions/20663619/what-does-amazon-aws-mean-by-network-performance#comment59356097_25620890
	//
	// It's a bit more complex than that, I'm afraid. - network links are more or less tiered by instance size, but with quite a bit of variation by generation and family.
	// "Low" is anywhere from 50 MBit to 300 MBit, "moderate" is 300-900 MBit (with fairly predictable numbers by instance type), "High" is 0.9-2.2 GBit.
	// [I did a metanalysis using public benchmarks](https://stackoverflow.com/questions/18507405/ec2-instance-typess-exact-network-performance/35806587#35806587).
	// BobMcGee
	// Mar 7, 2016 at 13:48
	//
	switch str {
	case "very low":
		*p = NetworkPerformance{Description: str, Range: Range[float64]{Unit: "Gbps", Min: 0, Max: 50.0 / 1000.0}}
		return nil
	case "low":
		*p = NetworkPerformance{Description: str, Range: Range[float64]{Unit: "Gbps", Min: 50.0 / 1000.0, Max: 300.0 / 1000.0}}
		return nil
	case "moderate":
		*p = NetworkPerformance{Description: str, Range: Range[float64]{Unit: "Gbps", Min: 300.0 / 1000.0, Max: 900.0 / 1000.0}}
		return nil
	case "low to moderate":
		*p = NetworkPerformance{Description: str, Range: Range[float64]{Unit: "Gbps", Min: 50.0 / 1000.0, Max: 900.0 / 1000.0}}
		return nil
	case "high":
		*p = NetworkPerformance{Description: str, Range: Range[float64]{Unit: "Gbps", Min: 900.0 / 1000.0, Max: 2200.0 / 1000.0}}
		return nil
	case "na":
		return nil
	case "":
		return nil
	}

	parts := strings.Split(str, " ")

	if len(parts) < 2 {
		*p = NetworkPerformance{Description: str, Range: Range[float64]{Min: -1, Max: -1}}
		return nil
	}

	*p = NetworkPerformance{
		Description: str,
		Range: Range[float64]{
			Unit: "Gbps",
		},
	}

	value := ParseFloat(parts[len(parts)-2], -1)
	p.Max = value
	p.Min = value

	if value == -1 {
		return nil
	}

	if parts[0] == "up" && parts[1] == "to" {
		p.Min = 0
	}

	unit := parts[len(parts)-1]
	if unit == "megabit" || unit == "mbps" {
		p.Min /= 1000
		p.Max /= 1000
	} else if unit != "gigabit" && unit != "gbps" {
		p = nil
	}

	return nil
}

//
// misc
//

func processPrices(prices *Prices) {
	for _, v := range prices.Terms {
		for _, terms := range v {
			for _, term := range terms {
				product, ok := prices.Products[term.SKU]
				if !ok {
					continue
				}
				term.Product = product
			}
		}
	}
}
