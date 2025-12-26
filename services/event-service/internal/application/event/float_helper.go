package event

import "strconv"

func strconvFormatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 8, 64)
}

func strconvParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
