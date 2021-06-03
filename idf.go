package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
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
	//stemmed word -> wordCount
	WordCounts map[string]int `json:"wordCounts"`
}

func (m *MessageWordIndex) TFIDF(index *IDFIndex) map[string]float64 {
	result := make(map[string]float64)
	idf := index.IDF()
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

func newMessageWordIndex(message *discordgo.Message) *MessageWordIndex {
	wc := make(map[string]int)

	words := extractWordsFromContent(message.Content)

	for _, word := range words {
		wc[word] += 1
	}

	return &MessageWordIndex{
		WordCounts: wc,
	}
}

//IDFIndex stores information for calculating IDF of a thread. Get a new one
//from NewIDFIndex.
type IDFIndex struct {
	//messageID --> *MessageWordIndex
	Messages map[string]*MessageWordIndex `json:"messages"`
	//channelID --> set of messageID
	MessagesForChannel map[string]map[string]bool `json:"messageForChannel"`
	idf                map[string]float64
}

func NewIDFIndex() *IDFIndex {
	return &IDFIndex{
		Messages:           make(map[string]*MessageWordIndex),
		MessagesForChannel: make(map[string]map[string]bool),
		//deliberately don't set idf, to signal it needs to be rebuilt.
	}
}

const CACHE_PATH = ".cache"
const IDF_CACHE_PATH = "idf"

//Persist persists the cache to disk. Load it back up later with guildID.
func (i *IDFIndex) Persist(guildID string) error {
	folderPath := filepath.Join(CACHE_PATH, IDF_CACHE_PATH)
	path := filepath.Join(folderPath, guildID+".json")
	blob, err := json.MarshalIndent(i, "", "\t")
	if err != nil {
		return fmt.Errorf("couldnt format json: %w", err)
	}
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		if err := os.MkdirAll(folderPath, 0700); err != nil {
			return fmt.Errorf("couldn't create cache folder: %w", err)
		}
	}
	return ioutil.WriteFile(path, blob, 0644)
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
	for _, messageIndex := range i.Messages {
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
	return len(i.Messages)
}

func (i *IDFIndex) MessageWordIndex(messageID string) *MessageWordIndex {
	return i.Messages[messageID]
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
	i.Messages[message.ID] = newMessageWordIndex(message)
	if _, ok := i.MessagesForChannel[message.ChannelID]; !ok {
		i.MessagesForChannel[message.ChannelID] = make(map[string]bool)
	}
	i.MessagesForChannel[message.ChannelID][message.ID] = true
}

//Computes a TFIDF sum for all messages in the given channel
func (i *IDFIndex) ChannelTFIDF(channelID string) map[string]float64 {
	result := make(map[string]float64)
	for messageID := range i.MessagesForChannel[channelID] {
		message := i.Messages[messageID]
		for key, val := range message.TFIDF(i) {
			result[key] += val
		}
	}
	return result
}
