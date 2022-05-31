package loadgen

import (
	"fmt"
	"testing"
)

func TestParseEntriesPerAdGenerator(t *testing.T) {
	type testCase struct {
		desc string
		typ  string
	}

	testCases := []testCase{
		{
			"Normal distribution. stdev=2, mean=100",
			"Normal(2, 100)",
		},
		{
			"Uniform distribution. start=20, end=100",
			"Uniform(20, 100)",
		},
		{
			"Always. val=20",
			"Always(20)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &Config{
				EntriesPerAdType: tc.typ,
			}

			c.ParseEntriesPerAdGenerator()
			fmt.Println(c.EntriesPerAdGenerator())
		})
	}

}
