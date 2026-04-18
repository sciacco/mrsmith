package energiadc

import (
	"math"
	"slices"
	"strings"
	"time"
)

const (
	localDateTimeLayout = "2006-01-02T15:04"
	sqlDateTimeLayout   = "2006-01-02 15:04:05"
	dateLayout          = "2006-01-02"
	dateTimeLayout      = "2006-01-02 15:04"
)

type ModuleConfig struct {
	ExcludedCustomerIDs []int
	Location            *time.Location
}

func normalizeConfig(cfg ModuleConfig) ModuleConfig {
	cfg.ExcludedCustomerIDs = slices.Clone(cfg.ExcludedCustomerIDs)
	if cfg.Location != nil {
		return cfg
	}

	location, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		location = time.FixedZone("Europe/Rome", 3600)
	}
	cfg.Location = location
	return cfg
}

func breakerCapacity(label string) float64 {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "trifase 32a":
		return 63
	case "monofase 16a":
		return 16
	default:
		return 32
	}
}

func gaugePercent(ampere, maxAmpere float64) float64 {
	if maxAmpere <= 0 {
		return 0
	}
	return roundFloat(ampere/(maxAmpere/2)*100, 1)
}

func cosfiMultiplier(percent int) float64 {
	return float64(percent) / 100
}

func kilowattFromAmpere(ampere float64) float64 {
	return roundFloat(ampere*225/1000, 3)
}

func roundFloat(value float64, precision int) float64 {
	factor := math.Pow(10, float64(precision))
	return math.Round(value*factor) / factor
}
