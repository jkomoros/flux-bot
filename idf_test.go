package main

import (
	"sort"
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
		"procrastination Procrastinate blarg baz the a is diamonds",
		"is is is a a a a is a the the the the the foo bar rare",
	}
	//TODO: are these really reasonable values for those inputs?
	expectedIDF := &idfIndexJSON{
		DocumentCount: len(inputs),
		DocumentWordCounts: map[string]int{
			"bar":        2,
			"baz":        2,
			"blarg":      1,
			"diamond":    1,
			"foo":        2,
			"procrastin": 2,
			"rare":       1,
		},
		FormatVersion: IDF_JSON_FORMAT_VERSION,
	}
	var messages []*discordgo.Message
	for i, input := range inputs {
		messages = append(messages, &discordgo.Message{
			Type:      discordgo.MessageTypeDefault,
			Content:   input,
			ID:        "Message " + strconv.Itoa(i),
			ChannelID: "DefaultChannel",
		})
	}
	index := newIDFIndex("invalid_guild_id")
	for _, message := range messages {
		index.ProcessMessage(message)
	}
	if index.DocumentCount() != len(inputs) {
		t.Errorf("Incorrect number of messages. Got %v, expected %v", index.DocumentCount(), len(inputs))
	}
	assert.For(t).ThatActual(index.data).Equals(expectedIDF).ThenDiffOnFail()

	tfidf := index.TFIDFForMessages(messages[1])
	expectedTFIDF := &TFIDF{
		values: map[string]float64{
			"baz":        0,
			"blarg":      0.17609125905568124,
			"diamond":    0.17609125905568124,
			"procrastin": 0,
		},
		messages: []*discordgo.Message{messages[1]},
	}
	assert.For(t).ThatActual(tfidf).Equals(expectedTFIDF)

	expectedChannelTFIDF := &TFIDF{
		values: map[string]float64{
			"bar":        0,
			"baz":        0,
			"blarg":      0.3521825181113625,
			"diamond":    0.3521825181113625,
			"foo":        0,
			"procrastin": 0,
			"rare":       0.17609125905568124,
		},
		messages: messages,
	}
	channelTFIDF := index.TFIDFForMessages(messages...)
	assert.For(t).ThatActual(channelTFIDF).Equals(expectedChannelTFIDF).ThenDiffOnFail()

	expectedTopWords := []string{
		"blarg",
		"diamonds",
		"rare",
	}
	actualTopWords := channelTFIDF.TopWords(3)
	sort.Strings(actualTopWords)
	assert.For(t).ThatActual(actualTopWords).Equals(expectedTopWords)

	expectedAutoTopWords := []string{
		"blarg",
		"diamonds",
	}

	actualAutoTopWords := channelTFIDF.AutoTopWords(6)
	sort.Strings(actualAutoTopWords)
	assert.For(t).ThatActual(actualAutoTopWords).Equals(expectedAutoTopWords)

}

func TestTFIDFTopWords(t *testing.T) {
	tfidf := &TFIDF{
		values: map[string]float64{
			"one":   0.5,
			"two":   1.0,
			"three": 0.25,
		},
	}
	assert.For(t).ThatActual(tfidf.TopWords(3)).Equals([]string{"two", "one", "three"})
	assert.For(t).ThatActual(tfidf.TopWords(2)).Equals([]string{"two", "one"})
	assert.For(t).ThatActual(tfidf.TopWords(4)).Equals([]string{"two", "one", "three"})

}
