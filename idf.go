package main

import (
	"math"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/dchest/stemmer/porter2"
)

var (
	spaceRegExp           *regexp.Regexp
	nonAlphaNumericRegExp *regexp.Regexp
)

func init() {
	spaceRegExp = regexp.MustCompile(`\s+`)
	nonAlphaNumericRegExp = regexp.MustCompile("[^a-zA-Z0-9]+")
}

type MessageWordIndex struct {
	//The Index this is part of
	Index     *IDFIndex
	wordCount int
	//stemmed word -> wordCount
	WordCounts map[string]int
}

func (m *MessageWordIndex) TFIDF() map[string]float64 {
	result := make(map[string]float64)
	idf := m.Index.IDF()
	for word, count := range m.WordCounts {
		result[word] = idf[word] * float64(count)
	}
	return result
}

func normalizeWord(input string) string {
	input = nonAlphaNumericRegExp.ReplaceAllString(input, "")
	input = porter2.Stemmer.Stem(input)
	return strings.ToLower(input)
}

func removeMentionsAndURLS(input string) string {
	//TODO: strip out markdown
	pieces := strings.Split(input, " ")
	var result []string
	for _, piece := range pieces {
		piece = strings.ToLower(piece)
		if strings.HasPrefix(piece, "https://") {
			continue
		}
		if strings.HasPrefix(piece, "http://") {
			continue
		}
		//Channel mentions look like <#837826557477126219>
		//User mentions look like <@!837476904742289429>
		if strings.HasPrefix(piece, "<") && strings.HasSuffix(piece, ">") {
			continue
		}
		result = append(result, piece)
	}
	return strings.Join(result, " ")
}

func wordsForString(input string) []string {
	input = strings.ReplaceAll(input, "-", " ")
	input = strings.ReplaceAll(input, "/", " ")
	return strings.Split(input, " ")
}

func extractWordsFromContent(input string) []string {
	//normalize all spaces to just a single space
	input = spaceRegExp.ReplaceAllString(input, " ")
	input = removeMentionsAndURLS(input)
	var result []string
	for _, word := range wordsForString(input) {
		word := normalizeWord(word)
		if word == "" {
			continue
		}
		result = append(result, word)
	}
	return result
}

func newMessageWordIndex(parent *IDFIndex, message *discordgo.Message) *MessageWordIndex {
	wc := make(map[string]int)

	words := extractWordsFromContent(message.Content)

	for _, word := range words {
		wc[word] += 1
	}

	return &MessageWordIndex{
		Index:      parent,
		wordCount:  len(words),
		WordCounts: wc,
	}
}

func (m *MessageWordIndex) WordCount() int {
	return m.wordCount
}

//IDFIndex stores information for calculating IDF of a thread. Get a new one
//from NewIDFIndex.
type IDFIndex struct {
	//channelID --> messageID --> *MessageWordIndex
	messages map[string]map[string]*MessageWordIndex
	//map of messageID to the channel it's in in the above
	channelIDForMessage map[string]string
	idf                 map[string]float64
}

func NewIDFIndex() *IDFIndex {
	return &IDFIndex{
		messages:            make(map[string]map[string]*MessageWordIndex),
		channelIDForMessage: make(map[string]string),
		//deliberately don't set idf, to signal it needs to be rebuilt.
	}
}

//Returns the Inverse Document Frequencey for the word in the corpus. Word may
//be stemmed or unstemmed.
func (i *IDFIndex) IDFForWord(word string) float64 {
	word = normalizeWord(word)
	return i.IDF()[word]
}

func (i *IDFIndex) IDF() map[string]float64 {
	if i.idf == nil {
		i.rebuildIDF()
	}
	return i.idf
}

func (i *IDFIndex) rebuildIDF() {
	//for each word, the number of messages that contain the word at least once.
	corpusWords := make(map[string]int)
	for _, channelCollection := range i.messages {
		for _, messageIndex := range channelCollection {
			for word := range messageIndex.WordCounts {
				corpusWords[word] += 1
			}
		}
	}
	idf := make(map[string]float64)

	numMessages := float64(i.DocumentCount())

	//idf (inverse document frequency) of every word in the corpus. See
	//https://en.wikipedia.org/wiki/Tf%E2%80%93idf
	for word, count := range corpusWords {
		idf[word] = math.Log10(numMessages / (float64(count) + 1))
	}
	i.idf = idf
}

func (i *IDFIndex) DocumentCount() int {
	return len(i.channelIDForMessage)
}

func (i *IDFIndex) MessageWordIndex(messageID string) *MessageWordIndex {
	channelID := i.channelIDForMessage[messageID]
	return i.messages[channelID][messageID]
}

//ProcessMessage will process a given message and update the index.
func (i *IDFIndex) ProcessMessage(message *discordgo.Message) {
	//TODO: test this
	if message == nil {
		return
	}
	//Skip messages that are not from users
	if message.Type != discordgo.MessageTypeDefault && message.Type != discordgo.MessageTypeReply {
		return
	}
	//Signal this needs to be reprocessed
	i.idf = nil
	i.channelIDForMessage[message.ID] = message.ChannelID
	if _, ok := i.messages[message.ChannelID]; !ok {
		i.messages[message.ChannelID] = make(map[string]*MessageWordIndex)
	}
	i.messages[message.ChannelID][message.ID] = newMessageWordIndex(i, message)
}

//Computes a TFIDF sum for all messages in the given channel
func (i *IDFIndex) ChannelTFIDF(channelID string) map[string]float64 {
	result := make(map[string]float64)
	for _, message := range i.messages[channelID] {
		for key, val := range message.TFIDF() {
			result[key] += val
		}
	}
	return result
}
