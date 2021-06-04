package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/workfit/tester/assert"
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
		{
			"Markdown",
			"foo **bar baz** _zing_",
			"foo bar baz zing",
		},
	}

	for i, test := range tests {
		result := strings.Join(extractWordsFromContent(test.Input), " ")
		if result != test.Expected {
			t.Errorf("Test %v %v : %v did not equal %v", i, test.Description, result, test.Expected)
		}
	}
}

func TestProcessMessage(t *testing.T) {
	inputs := []string{
		"the the the foo bar baz is a procrastinate",
		"procrastination Procrastinate blarg baz the a is diamond",
		"is is is a a a a is a the the the the the foo bar rare",
	}
	//TODO: are these really reasonable values for those inputs?
	expectedIDF := map[string]float64{
		"a":          -0.12493873660829993,
		"bar":        0,
		"baz":        0,
		"blarg":      0.17609125905568124,
		"diamond":    0.17609125905568124,
		"foo":        0,
		"is":         -0.12493873660829993,
		"procrastin": 0,
		"rare":       0.17609125905568124,
		"the":        -0.12493873660829993,
	}
	index := NewIDFIndex()
	for i, message := range inputs {
		index.ProcessMessage(&discordgo.Message{
			Type:      discordgo.MessageTypeDefault,
			Content:   message,
			ID:        "Message " + strconv.Itoa(i),
			ChannelID: "DefaultChannel",
		})
	}
	if index.DocumentCount() != len(inputs) {
		t.Errorf("Incorrect number of messages. Got %v, expected %v", index.DocumentCount(), len(inputs))
	}
	idf := index.IDF()
	assert.For(t).ThatActual(idf).Equals(expectedIDF).ThenDiffOnFail()

	messageIndex := index.MessageWordIndex("Message 1")
	assert.For(t).ThatActual(messageIndex).IsNotNil()
	tfidf := messageIndex.TFIDF(index)
	expectedTFIDF := map[string]float64{
		"a":          -0.12493873660829993,
		"baz":        0,
		"blarg":      0.17609125905568124,
		"diamond":    0.17609125905568124,
		"is":         -0.12493873660829993,
		"procrastin": 0,
		"the":        -0.12493873660829993,
	}
	assert.For(t).ThatActual(map[string]float64(tfidf)).Equals(expectedTFIDF)

	expectedChannelTFIDF := map[string]float64{
		"a":          -0.8745711562580996,
		"bar":        0,
		"baz":        0,
		"blarg":      0.17609125905568124,
		"diamond":    0.17609125905568124,
		"foo":        0,
		"is":         -0.7496324196497997,
		"procrastin": 0,
		"rare":       0.17609125905568124,
		"the":        -1.1244486294746994,
	}
	assert.For(t).ThatActual(map[string]float64(index.ChannelTFIDF("DefaultChannel"))).Equals(expectedChannelTFIDF)

}
