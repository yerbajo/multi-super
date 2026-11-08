package mercadona

import (
	"strconv"
	"strings"
)

// The store publishes three numbers per product and this file turns them into the
// contract's two: what the package costs and what a unit of it costs. Nothing is divided
// by us: reference_price and reference_format are the €/unit printed on the shelf, so a
// format this vocabulary cannot name leaves both fields empty instead of inventing one.

// price reads unit_price, the price of the package as sold.
func (p priceInstructions) price() *float64 {
	v, ok := parseMoney(p.UnitPrice)
	if !ok {
		return nil
	}
	return &v
}

// unitPrice maps reference_price + reference_format onto the contract vocabulary
// (kg, L, unit). A format the contract cannot express ("100 g", whose value the store
// publishes inconsistently) yields no unit price at all: a €/unit without its unit is a
// number nobody can compare (T21).
func (p priceInstructions) unitPrice() (float64, string) {
	v, ok := parseMoney(p.ReferencePrice)
	if !ok {
		return 0, ""
	}
	switch strings.ToLower(strings.TrimSpace(p.ReferenceFormat)) {
	case "kg":
		return v, "kg"
	case "l":
		return v, "L"
	case "ud", "uds", "unidad", "unidades":
		return v, "unit"
	}
	return 0, ""
}

// size composes the sold size from the two published numbers ("6 l", "0.2 kg"). It is
// the whole package, pack multiplier included: unit_size already carries the total.
func (p priceInstructions) size() string {
	format := strings.TrimSpace(p.SizeFormat)
	if p.UnitSize <= 0 || format == "" {
		return ""
	}
	return strconv.FormatFloat(p.UnitSize, 'f', -1, 64) + " " + format
}

// parseMoney reads a decimal amount the store publishes as text ("5.76", "1,43").
func parseMoney(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
