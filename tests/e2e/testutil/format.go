package testutil

import "strings"

// Lines joins each element with "\n" (no leading or trailing newline added).
// Use to build multi-line golden output one source line per output line.
// To represent a trailing newline at the end of stdout, append "" as the
// last element:
//
//	Lines("first", "second", "")        // => "first\nsecond\n"
//	Lines("first", "", "third", "")     // => "first\n\nthird\n"
func Lines(ls ...string) string {
	return strings.Join(ls, "\n")
}

// Cols joins each cell with "\t". Use for tab-separated rows (the format
// `taskgate show` uses for path + summary columns).
//
//	Cols(".taskgate/human/build", "Build the project.")  // => ".taskgate/human/build\tBuild the project."
func Cols(cells ...string) string {
	return strings.Join(cells, "\t")
}
