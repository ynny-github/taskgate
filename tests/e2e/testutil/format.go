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

// Cols joins each cell with aligned spacing. The first cell is right-padded
// to align all second cells. Use for the format `taskgate show` uses for
// path/name + summary columns.
//
//	Cols(".taskgate/human/build", "Build the project.")  // => ".taskgate/human/build  Build the project."
func Cols(cells ...string) string {
	if len(cells) < 2 {
		return strings.Join(cells, "  ")
	}
	// Align second column by padding first cell to longest width + 2 spaces
	padding := len(cells[0]) + 2
	return cells[0] + strings.Repeat(" ", padding-len(cells[0])) + cells[1]
}
