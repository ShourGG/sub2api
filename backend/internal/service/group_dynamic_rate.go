package service

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

const (
	DefaultDynamicRateMarkup = 1.25
	DynamicRateRoundingStep  = 0.005
)

func validateDynamicRateMarkup(markup float64) error {
	if markup <= 0 || math.IsNaN(markup) || math.IsInf(markup, 0) {
		return fmt.Errorf("dynamic_rate_markup must be a finite number > 0")
	}
	return nil
}

// CalculateDynamicGroupRate converts an upstream source multiplier to the
// public group multiplier. Decimal arithmetic keeps exact half-step values
// such as 0.0875 from being rounded differently across platforms.
func CalculateDynamicGroupRate(sourceMultiplier, markup float64) (float64, error) {
	if sourceMultiplier < 0 || math.IsNaN(sourceMultiplier) || math.IsInf(sourceMultiplier, 0) {
		return 0, fmt.Errorf("dynamic source multiplier must be a finite number >= 0")
	}
	if err := validateDynamicRateMarkup(markup); err != nil {
		return 0, err
	}
	source := decimal.NewFromFloat(sourceMultiplier)
	markupDecimal := decimal.NewFromFloat(markup)
	step := decimal.NewFromFloat(DynamicRateRoundingStep)
	return source.Mul(markupDecimal).Div(step).Ceil().Mul(step).InexactFloat64(), nil
}
