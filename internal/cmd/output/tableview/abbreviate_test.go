package tableview

import "testing"

func TestAbbreviateMatrixIDs(t *testing.T) {
	headers := []string{"ID", "Name"}
	rows := [][]string{
		{"94db15db-4d02-46c2-a007-765e7a1d64c7", "example"},
		{"12345678-1234-1234-1234-1234567890ab", "other"},
	}

	abbr := abbreviateMatrixIDs(headers, rows)

	want := []string{"94db…", "1234…"}
	for i, row := range abbr {
		if row[0] != want[i] {
			t.Fatalf("row %d ID = %q, want %q", i, row[0], want[i])
		}
	}

	// ensure original slice not modified
	if rows[0][0] == abbr[0][0] {
		t.Fatalf("expected input matrix to remain unchanged")
	}
}
