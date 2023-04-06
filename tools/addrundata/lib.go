package main

import (
	"regexp"
	"strconv"
)

var versionTokenRE = regexp.MustCompile(`(\D*)(\d*)`)

// lessVersion performs a less-than comparison by splitting non-digit and digit tokens
// from the strings.  Digits are compared as integers.
func lessVersion(x, y string) bool {
	xs := versionTokenRE.FindAllStringSubmatch(x, -1)
	ys := versionTokenRE.FindAllStringSubmatch(y, -1)

	for i := 0; i < len(xs) && i < len(ys); i++ {
		xword, xdigit := xs[i][1], xs[i][2]
		yword, ydigit := ys[i][1], ys[i][2]

		switch {
		case xword < yword:
			return true
		case xword > yword:
			return false
		}

		xn, xerr := strconv.Atoi(xdigit)
		yn, yerr := strconv.Atoi(ydigit)

		// First try to compare x and y as numbers.
		if xerr == nil && yerr == nil {
			switch {
			case xn < yn:
				return true
			case xn > yn:
				return false
			}
		}

		// Compare them as string, so e.g. '01' < '1'.
		switch {
		case xdigit < ydigit:
			return true
		case xdigit > ydigit:
			return false
		}
	}

	return len(xs) < len(ys)
}
