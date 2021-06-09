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
	var messages []*discordgo.Message
	for i, input := range inputs {
		messages = append(messages, &discordgo.Message{
			Type:      discordgo.MessageTypeDefault,
			Content:   input,
			ID:        "Message " + strconv.Itoa(i),
			ChannelID: "DefaultChannel",
		})
	}
	index := NewIDFIndex("invalid_guild_id")
	for _, message := range messages {
		index.ProcessMessage(message)
	}
	if index.DocumentCount() != len(inputs) {
		t.Errorf("Incorrect number of messages. Got %v, expected %v", index.DocumentCount(), len(inputs))
	}
	idf := index.IDF()
	assert.For(t).ThatActual(idf).Equals(expectedIDF).ThenDiffOnFail()

	messageIndex := index.MessageWordIndex("Message 1")
	assert.For(t).ThatActual(messageIndex).IsNotNil()
	tfidf := messageIndex.TFIDF(index)
	expectedTFIDF := &TFIDF{
		values: map[string]float64{
			"a":          -0.12493873660829993,
			"baz":        0,
			"blarg":      0.17609125905568124,
			"diamond":    0.17609125905568124,
			"is":         -0.12493873660829993,
			"procrastin": 0,
			"the":        -0.12493873660829993,
		},
		messages: []*MessageWordIndex{index.data.Messages["Message 1"]},
	}
	assert.For(t).ThatActual(tfidf).Equals(expectedTFIDF)

	expectedChannelTFIDF := &TFIDF{
		values: map[string]float64{
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
		},
		messages: []*MessageWordIndex{
			index.data.Messages["Message 0"],
			index.data.Messages["Message 1"],
			index.data.Messages["Message 2"],
		},
	}
	channelTFIDF := index.ChannelTFIDF("DefaultChannel")
	//Give messages a stable sort order
	sort.Slice(channelTFIDF.messages, func(i, j int) bool {
		return channelTFIDF.messages[i].Message.ID < channelTFIDF.messages[j].Message.ID
	})
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
		"rare",
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
