package main

import (
	"strings"
	"testing"
)

func TestExtractWordsFromContent(t *testing.T) {
	tests := []struct {
		Description string
		Input       string
		Expected    string
	}{
		{
			"No op",
			"noop test",
			"noop test",
		},
		//TODO: many more tests, including exercising all things covered by a TODO in idf.go
	}

	for i, test := range tests {
		result := strings.Join(extractWordsFromContent(test.Input), " ")
		if result != test.Expected {
			t.Errorf("Test %v %v : %v did not equal %v", i, test.Description, result, test.Expected)
		}
	}
}
