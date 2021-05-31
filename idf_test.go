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
		{
			"Multiple types of whitespace",
			"foo	foo\n foo foo",
			"foo foo foo foo",
		},
		{
			"Lowercase",
			"LoWERcase",
			"lowercas",
		},
		{
			"URLs stripped",
			"foo https://www.example.com/foo/?foo=foo foo",
			"foo foo",
		},
		{
			"Person mentions stripped",
			"foo <@!837476904742289429> foo",
			"foo foo",
		},
		{
			"Channel mentions stripped",
			"foo <#837826557477126219> foo",
			"foo foo",
		},
		{
			"Dashes as spaces",
			"foo-bar foo",
			"foo bar foo",
		},
		{
			"Slashes as spaces",
			"foo/bar foo",
			"foo bar foo",
		},
		{
			"Punctuation stripped",
			"foo & (foo)!",
			"foo foo",
		},
		{
			"Stemming",
			"procrastination",
			"procrastin",
		},
	}

	for i, test := range tests {
		result := strings.Join(extractWordsFromContent(test.Input), " ")
		if result != test.Expected {
			t.Errorf("Test %v %v : %v did not equal %v", i, test.Description, result, test.Expected)
		}
	}
}
