/*
	InfluxDB client
	(c) Copyright David Thorpe 2017
	All Rights Reserved

	For Licensing and Usage information, please see LICENSE file
*/

package influxdb

import (
	"regexp"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS & CONSTS

var (
	regexpBareIdentifier = regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*$")
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Quote returns a query-safe version of an identifier (a database, series
// or measurement name)
func Quote(value string) string {
	if value == "" {
		return value
	} else if isBareIdentifier(value) {
		return value
	} else {
		return "\"" + escapeString(value) + "\""
	}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func isBareIdentifier(value string) bool {
	return regexpBareIdentifier.MatchString(value)
}

// Append \ to every newline double quote and backslash
func escapeString(value string) string {
	value = strings.Replace(value, "\\", "\\\\", -1)
	value = strings.Replace(value, "\"", "\\\"", -1)
	return value
}
