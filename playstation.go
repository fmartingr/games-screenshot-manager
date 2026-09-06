package gamesscreenshotmanager

import (
	"fmt"
	"strings"
	"time"
)

const (
	// A capture the console named after a scene holds no time, as in
	// "Main Menu.jpg". The file keeps its own name under this folder, below the
	// game folder, so that it stays out of the dated set.
	playstationUndatedFolder = "Other"
	playstationUndatedPrefix = "Undated_"
)

// parsePlaystationDatetime reads the capture time out of a PlayStation file
// name. name is the file name without its extension.
//
// The name ends with a timestamp, and two things can follow it:
//
//   - A "_<n>" counter. The console appends one to a file it copies into a
//     folder that already holds that name. A whole game album copied twice onto
//     one USB drive arrives as such a set.
//   - Digits below the second. Some titles write more digits than the layout
//     holds, as in "SPACE RUN_2024081217012400".
//
// The second return value is true when the name carries a counter.
func parsePlaystationDatetime(name, layout string) (time.Time, bool, error) {
	// The counter is a run of digits, and so is the timestamp in front of it.
	// A counter therefore counts as one only when a timestamp stays behind
	// after the cut. "Ghost of Tsushima_20260831083523" ends in digits too.
	if index := strings.LastIndex(name, "_"); index >= 0 && isDigits(name[index+1:]) {
		if datetime, err := parseTailDatetime(name[:index], layout); err == nil {
			return datetime, true, nil
		}
	}

	datetime, err := parseTailDatetime(name, layout)

	return datetime, false, err
}

// parseTailDatetime reads a timestamp off the end of name. It takes the run of
// digits the name ends with, and it reads the layout from the front of that
// run. The digits after the layout are below the second, so they are dropped.
func parseTailDatetime(name, layout string) (time.Time, error) {
	digits := 0
	for digits < len(name) && isDigits(name[len(name)-digits-1:len(name)-digits]) {
		digits++
	}

	if digits < len(layout) {
		return time.Time{}, fmt.Errorf("the name %q does not end with a timestamp of %d digits", name, len(layout))
	}

	start := len(name) - digits

	return time.Parse(layout, name[start:start+len(layout)])
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}

	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}
