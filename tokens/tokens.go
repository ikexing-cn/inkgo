// Package tokens writes and reads counts of tokens as the author writes
// them: 128K, 1M, with K a thousand and M a million.
package tokens

import (
	"math"
	"strconv"
	"strings"
)

// Format writes n with M for whole millions and K for whole thousands, as
// 1M and 128K; any other count stays in digits.
func Format(n int) string {
	switch {
	case n != 0 && n%1_000_000 == 0:
		return strconv.Itoa(n/1_000_000) + "M"
	case n != 0 && n%1000 == 0:
		return strconv.Itoa(n/1000) + "K"
	}
	return strconv.Itoa(n)
}

// Parse reads a count as Format writes it, in either case, with spaces
// before the unit, or with a fraction of the unit, as 1.5M. It fails on
// anything else, on a fraction finer than one token, and on counts below
// one.
func Parse(s string) (int, bool) {
	s = strings.TrimSpace(s)
	unit := 1
	if s != "" {
		switch s[len(s)-1] {
		case 'k', 'K':
			unit = 1000
		case 'm', 'M':
			unit = 1_000_000
		}
		if unit > 1 {
			s = strings.TrimSpace(s[:len(s)-1])
		}
	}
	whole, frac, dotted := strings.Cut(s, ".")
	if !digits(whole) || len(whole) > 12 || dotted && !digits(frac) {
		return 0, false
	}
	scale := 1
	for range frac {
		scale *= 10
	}
	if unit%scale != 0 {
		return 0, false
	}
	w, _ := strconv.Atoi(whole)
	f := 0
	if frac != "" {
		f, _ = strconv.Atoi(frac)
	}
	n := w*unit + f*(unit/scale)
	return n, n >= 1
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Round writes n to be read rather than typed: from a thousand on, to one
// decimal place of K, M or B, as 9.6K and 1.7M; below, in digits.
func Round(n int) string {
	for _, u := range []struct {
		unit string
		by   float64
	}{{"B", 1e9}, {"M", 1e6}, {"K", 1e3}} {
		if tenths := math.Round(float64(n) / u.by * 10); n >= 1000 && tenths >= 10 {
			return strconv.FormatFloat(tenths/10, 'f', -1, 64) + u.unit
		}
	}
	return strconv.Itoa(n)
}
