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
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dchest/stemmer/porter2"
)

var (
	spaceRegExp           *regexp.Regexp
	nonAlphaNumericRegExp *regexp.Regexp
)

const (
	CACHE_PATH     = ".cache"
	IDF_CACHE_PATH = "idf"
)

const AUTO_SAVE_INTERVAL = 5 * time.Minute

//This number should be incremetned every time the format of the JSON cache
//changes, so old caches will be discarded.
const IDF_JSON_FORMAT_VERSION = 2

func init() {
	spaceRegExp = regexp.MustCompile(`\s+`)
	nonAlphaNumericRegExp = regexp.MustCompile("[^a-zA-Z0-9]+")
}

type TFIDF map[string]float64

//Effectively a subset of discordgo.Message with only the fields we want.
type Message struct {
	ID              string                         `json:"id"`
	Content         string                         `json:"content"`
	Timestamp       discordgo.Timestamp            `json:"timestamp"`
	EditedTimestamp discordgo.Timestamp            `json:"edited_timestamp"`
	Attachments     []*discordgo.MessageAttachment `json:"attachments"`
	Reactions       []*discordgo.MessageReactions  `json:"reactions"`
	Type            discordgo.MessageType          `json:"type"`

	//These are ID fields for the items that would be too large to output multiple times
	//Author
	AuthorID string `json:"author_id"`
	//Mentions
	MentionUserIDs []string `json:"mention_user_ids"`
	//MentionChannels
	MentionChannelIDs []string `json:"mention_channel_ids"`
}

func messageFromDiscordMessage(input *discordgo.Message) *Message {
	userIDs := make([]string, 0)
	for _, user := range input.Mentions {
		userIDs = append(userIDs, user.ID)
	}
	channelIDs := make([]string, 0)
	for _, channel := range input.MentionChannels {
		channelIDs = append(channelIDs, channel.ID)
	}
	var authorID string
	if input.Author != nil {
		authorID = input.Author.ID
	}
	return &Message{
		ID:                input.ID,
		Content:           input.Content,
		Timestamp:         input.Timestamp,
		EditedTimestamp:   input.EditedTimestamp,
		Attachments:       input.Attachments,
		Reactions:         input.Reactions,
		Type:              input.Type,
		AuthorID:          authorID,
		MentionUserIDs:    userIDs,
		MentionChannelIDs: channelIDs,
	}
}

type MessageWordIndex struct {
	Message *Message `json:"message"`
	//stemmed word -> wordCount
	WordCounts map[string]int `json:"wordCounts"`
}

func (m *MessageWordIndex) TFIDF(index *IDFIndex) TFIDF {
	result := make(TFIDF)
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
		Message:    messageFromDiscordMessage(message),
		WordCounts: wc,
	}
}

type idfIndexJSON struct {
	//messageID --> *MessageWordIndex
	Messages map[string]*MessageWordIndex `json:"messages"`
	//channelID --> set of messageID
	MessagesForChannel map[string]map[string]bool `json:"messageForChannel"`
	FormatVersion      int                        `json:"formatVersion"`
}

//IDFIndex stores information for calculating IDF of a thread. Get a new one
//from NewIDFIndex.
type IDFIndex struct {
	data    *idfIndexJSON
	guildID string
	idf     map[string]float64
	//set if there are changes made since the last time we persisted
	dirty         bool
	autoSaveTimer *time.Timer
	rwMutex       sync.RWMutex
}

//IDFIndexForGuild returns either a preexisting IDF index from disk cache or a
//fresh one.z
func IDFIndexForGuild(guildID string) *IDFIndex {
	if result := LoadIDFIndex(guildID); result != nil {
		return result
	}
	return NewIDFIndex(guildID)
}

func LoadIDFIndex(guildID string) *IDFIndex {
	folderPath := filepath.Join(CACHE_PATH, IDF_CACHE_PATH)
	path := filepath.Join(folderPath, guildID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	blob, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("couldn't read json file for %v: %v", guildID, err)
		return nil
	}
	var result idfIndexJSON
	if err := json.Unmarshal(blob, &result); err != nil {
		fmt.Printf("couldn't unmarshal json for %v: %v", guildID, err)
		return nil
	}
	if result.FormatVersion != IDF_JSON_FORMAT_VERSION {
		fmt.Printf("%v IDF cache file had old version %v, expected %v, discarding\n", guildID, result.FormatVersion, IDF_JSON_FORMAT_VERSION)
		return nil
	}
	fmt.Printf("Reloading guild IDF cachce for %v\n", guildID)
	return &IDFIndex{
		data: &result,
		idf:  nil,
	}
}

func NewIDFIndex(guildID string) *IDFIndex {
	data := &idfIndexJSON{
		Messages:           make(map[string]*MessageWordIndex),
		MessagesForChannel: make(map[string]map[string]bool),
		FormatVersion:      IDF_JSON_FORMAT_VERSION,
	}
	return &IDFIndex{
		data:    data,
		guildID: guildID,
		//deliberately don't set idf, to signal it needs to be rebuilt.
	}
}

//Returns true if there's state not yet persisted
func (i *IDFIndex) NeedsPersistence() bool {
	if i.dirty {
		return true
	}
	folderPath := filepath.Join(CACHE_PATH, IDF_CACHE_PATH)
	path := filepath.Join(folderPath, i.guildID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true
	}
	return false
}

func (i *IDFIndex) PersistIfNecessary() error {
	if !i.NeedsPersistence() {
		return nil
	}
	return i.Persist()
}

func (i *IDFIndex) setNeedsPersistence() {
	i.dirty = true
	if i.autoSaveTimer != nil {
		return
	}
	i.autoSaveTimer = time.AfterFunc(AUTO_SAVE_INTERVAL, i.autoSave)
}

func (i *IDFIndex) setPersisted() {
	i.dirty = false
	if i.autoSaveTimer != nil {
		i.autoSaveTimer.Stop()
		i.autoSaveTimer = nil
	}
}

func (i *IDFIndex) autoSave() {
	//Don't print the autosave message unless we're actually persisting.
	if !i.NeedsPersistence() {
		return
	}
	fmt.Printf("Autosaving index for guild %v\n", i.guildID)
	if err := i.PersistIfNecessary(); err != nil {
		fmt.Printf("Error: couldn't autosave: %v\n", err)
	}
}

//Persist persists the cache to disk. Load it back up later with guildID.
func (i *IDFIndex) Persist() error {
	folderPath := filepath.Join(CACHE_PATH, IDF_CACHE_PATH)
	path := filepath.Join(folderPath, i.guildID+".json")
	i.rwMutex.RLock()
	blob, err := json.MarshalIndent(i.data, "", "\t")
	i.rwMutex.RUnlock()
	if err != nil {
		return fmt.Errorf("couldnt format json: %w", err)
	}
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		if err := os.MkdirAll(folderPath, 0700); err != nil {
			return fmt.Errorf("couldn't create cache folder: %w", err)
		}
	}
	i.setPersisted()
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
	for _, messageIndex := range i.data.Messages {
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
	return len(i.data.Messages)
}

func (i *IDFIndex) MessageWordIndex(messageID string) *MessageWordIndex {
	return i.data.Messages[messageID]
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
	i.rwMutex.Lock()
	i.idf = nil
	i.data.Messages[message.ID] = newMessageWordIndex(message)
	if _, ok := i.data.MessagesForChannel[message.ChannelID]; !ok {
		i.data.MessagesForChannel[message.ChannelID] = make(map[string]bool)
	}
	i.data.MessagesForChannel[message.ChannelID][message.ID] = true
	i.setNeedsPersistence()
	i.rwMutex.Unlock()
}

//Computes a TFIDF sum for all messages in the given channel
func (i *IDFIndex) ChannelTFIDF(channelID string) TFIDF {
	result := make(map[string]float64)
	for messageID := range i.data.MessagesForChannel[channelID] {
		message := i.data.Messages[messageID]
		for key, val := range message.TFIDF(i) {
			result[key] += val
		}
	}
	return result
}
