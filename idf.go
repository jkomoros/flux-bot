package main

import (
	"math"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var spaceRegExp *regexp.Regexp

func init() {
	spaceRegExp = regexp.MustCompile(`\s+`)
}

type MessageWordIndex struct {
	//The Index this is part of
	Index     *IDFIndex
	wordCount int
	//stemmed word -> wordCount
	WordCounts map[string]int
}

func normalizeWord(input string) string {
	//TODO: test this function
	//TODO: stem
	//TODO: strip out punctuation
	return strings.ToLower(input)
}

func removeMentionsAndURLS(input string) string {
	//TODO: test this function
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
	return strings.Join(pieces, " ")
}

func wordsForString(input string) []string {
	input = strings.ReplaceAll(input, "-", " ")
	input = strings.ReplaceAll(input, "/", " ")
	return strings.Split(input, " ")
}

func extractWordsFromContent(input string) []string {
	//TODO: test this function
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
	//messageID --> *MessageWordIndex
	messages map[string]*MessageWordIndex
	idf      map[string]float64
}

func NewIDFIndex() *IDFIndex {
	return &IDFIndex{
		messages: make(map[string]*MessageWordIndex),
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
	for _, messageIndex := range i.messages {
		for word := range messageIndex.WordCounts {
			corpusWords[word] += 1
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
	return len(i.messages)
}

func (i *IDFIndex) MessageWordIndex(messageID string) *MessageWordIndex {
	return i.messages[messageID]
}

//ProcessMessage will process a given message and update the index.
func (i *IDFIndex) ProcessMessage(message *discordgo.Message) {
	if message == nil {
		return
	}
	//Skip messages that are not from users
	if message.Type != discordgo.MessageTypeDefault && message.Type != discordgo.MessageTypeReply {
		return
	}
	//Signal this needs to be reprocessed
	i.idf = nil
	i.messages[message.ID] = newMessageWordIndex(i, message)
}
