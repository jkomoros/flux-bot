package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const FORK_THREAD_EMOJI = "🧵"
const START_FORK_THREAD_EMOJI = "🪡"

type categoryMap map[string]*threadGroupInfo

type bot struct {
	session    *discordgo.Session
	controller Controller
	//guildID -> threadCategoryChannelID -> info
	infos           map[string]categoryMap
	infoMutex       sync.RWMutex
	indexes         map[string]*IDFIndex
	rebuildIDFTimer *time.Timer
}

type threadGroupInfo struct {
	name                     string
	threadCategoryID         string
	nextArchiveCategoryIndex int
	activeArchiveCategoryID  string
	archiveCategoryIDs       []string
}

type byArchiveIndex []*discordgo.Channel

//Sort in a similar way as the main discord client
type byDiscordOrder []*discordgo.Channel

func messageReference(guildID, channelID, messageID string) *discordgo.MessageReference {
	return &discordgo.MessageReference{
		GuildID:   guildID,
		ChannelID: channelID,
		MessageID: messageID,
	}
}

func newBot(s *discordgo.Session, c Controller) *bot {
	result := &bot{
		session:    s,
		controller: c,
		infos:      make(map[string]categoryMap),
		indexes:    make(map[string]*IDFIndex),
	}
	s.AddHandler(result.ready)
	s.AddHandler(result.guildCreate)
	s.AddHandler(result.messageCreate)
	s.AddHandler(result.messageUpdate)
	s.AddHandler(result.messageDelete)
	s.AddHandler(result.messageDeleteBulk)
	s.AddHandler(result.channelCreate)
	s.AddHandler(result.channelUpdate)
	s.AddHandler(result.channelDelete)
	s.AddHandler(result.messageReactionAdd)
	s.AddHandler(result.messageReactionRemove)
	s.AddHandler(result.messageReactionsRemoveAll)
	s.AddHandler(result.interactionCreate)
	return result
}

func (b *bot) start() error {
	if err := b.registerSlashCommands(); err != nil {
		return fmt.Errorf("couldn't register slash commands: %v", err)
	}
	b.scheduleRebuildIDFCache()
	return nil
}

//registerSlashCommands must be called after the bot is already connected
func (b *bot) registerSlashCommands() error {
	for _, v := range commands {
		//debugGuildIDForCommand will be "" (global) in the common case, only set during development.
		_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, debugGuildIDForCommand, v)
		if err != nil {
			return fmt.Errorf("couldn't register command %v: %w", v.Name, err)
		}
	}
	return nil
}

// discordgo callback: called when the bot receives the "ready" event from Discord.
func (b *bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	//GuildInfo isn't populated yet.
	fmt.Println("Ready and waiting!")
}

// discordgo callback: called after the bot starts up for each guild it's added to
func (b *bot) guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	b.setGuildNeedsInfoRegeneration(event.Guild.ID)
	guildInfos := b.getInfos(event.Guild.ID)
	if guildInfos == nil {
		fmt.Printf("Couldn't find guild with ID %v\n", event.Guild.ID)
	}
	for _, group := range guildInfos {
		if err := group.archiveThreadsIfNecessary(b.controller, b.session); err != nil {
			fmt.Printf("Couldn't archive extra threads on boot: %v\n", err)
		}
	}
	//ensure that an IDF index exists, or build it now so we'll have it if we need it
	idf, err := IDFIndexForGuild(event.Guild.ID, s)
	if err != nil {
		fmt.Printf("couldn't fetch idf for guild %v: %v\n", event.Guild.ID, err)
	}
	b.indexes[event.Guild.ID] = idf
}

// discordgo callback: called after the when new message is posted.
func (b *bot) messageCreate(s *discordgo.Session, event *discordgo.MessageCreate) {

	if err := b.noteMessageIfFork(event.Message); err != nil {
		fmt.Printf("couldn't note forked message: %v\n", err)
	}

	channel, err := s.State.Channel(event.ChannelID)
	if err != nil {
		fmt.Println("Couldn't find channel")
		return
	}
	if !b.isThread(channel) {
		return
	}
	if err := b.moveThreadToTopOfThreads(channel); err != nil {
		fmt.Printf("message received in a thread but couldn't move it: %v\n", err)
	}
}

// discordgo callback: called after the when a message is edited
func (b *bot) messageUpdate(s *discordgo.Session, event *discordgo.MessageUpdate) {
	if err := b.updateForkedMessages(event.Message); err != nil {
		fmt.Printf("couldn't update forked messages if any existed: %v\n", err)
	}
}

// discordgo callback: called after the when a message is edited
func (b *bot) messageDelete(s *discordgo.Session, event *discordgo.MessageDelete) {
	idf, err := b.getLiveIDFIndex(event.GuildID)
	if err != nil {
		fmt.Printf("couldn't get idf index: %v\n", err)
		return
	}
	idf.NoteMessageDeleted(event.Message.ID)
}

// discordgo callback: called after the when a message is edited
func (b *bot) messageDeleteBulk(s *discordgo.Session, event *discordgo.MessageDeleteBulk) {
	idf, err := b.getLiveIDFIndex(event.GuildID)
	if err != nil {
		fmt.Printf("couldn't get idf index: %v\n", err)
		return
	}
	for _, msgID := range event.Messages {
		idf.NoteMessageDeleted(msgID)
	}
}

// discordgo callback: called after new channel is created.
func (b *bot) channelCreate(s *discordgo.Session, event *discordgo.ChannelCreate) {
	b.setGuildNeedsInfoRegeneration(event.GuildID)

	channel := event.Channel
	if !b.isThread(channel) {
		return
	}
	if err := b.moveThreadToTopOfThreads(channel); err != nil {
		fmt.Printf("message received in a thread but couldn't move it: %v\n", err)
	}
	guildInfos := b.getInfos(event.GuildID)
	if guildInfos == nil {
		fmt.Println("Couldnt get guild info to archive if necessary")
		return
	}
	for _, group := range guildInfos {
		if err := group.archiveThreadsIfNecessary(b.controller, b.session); err != nil {
			fmt.Printf("Couldn't archive threads if necessary: %v\n", err)
		}
	}
}

// discordgo callback: channelUpdate happens a LOT, e.g. every time we reorder a channel, every
// single channel whose index changed will get called one at a time.
func (b *bot) channelUpdate(s *discordgo.Session, event *discordgo.ChannelUpdate) {
	b.setGuildNeedsInfoRegeneration(event.GuildID)
}

// discordgo callback: called after the when a message is edited
func (b *bot) channelDelete(s *discordgo.Session, event *discordgo.ChannelDelete) {
	idf, err := b.getLiveIDFIndex(event.GuildID)
	if err != nil {
		fmt.Printf("couldn't get idf index: %v\n", err)
		return
	}
	idf.NoteChannelDeleted(event.Channel.ID)
}

func (b *bot) messageReactionAdd(s *discordgo.Session, event *discordgo.MessageReactionAdd) {
	ref := messageReference(event.GuildID, event.ChannelID, event.MessageID)
	switch event.Emoji.Name {
	case FORK_THREAD_EMOJI:
		if err := b.forkThreadViaEmojiToNewThread(ref, event.UserID); err != nil {
			fmt.Printf("couldn't fork thread: %v\n", err)
		}
	default:
		if err := b.updateForkedMessagesIfTheyExist(ref); err != nil {
			fmt.Printf("Couldn't update forks if they exist: %v\n", err)
		}
	}
}

func (b *bot) messageReactionRemove(s *discordgo.Session, event *discordgo.MessageReactionRemove) {
	ref := messageReference(event.GuildID, event.ChannelID, event.MessageID)
	if err := b.updateForkedMessagesIfTheyExist(ref); err != nil {
		fmt.Printf("Couldn't update forks if they exist: %v\n", err)
	}
}

func (b *bot) messageReactionsRemoveAll(s *discordgo.Session, event *discordgo.MessageReactionRemoveAll) {
	ref := messageReference(event.GuildID, event.ChannelID, event.MessageID)
	if err := b.updateForkedMessagesIfTheyExist(ref); err != nil {
		fmt.Printf("Couldn't update forks if they exist: %v\n", err)
	}
}

func (b *bot) forkThreadViaEmojiToNewThread(ref *discordgo.MessageReference, userID string) error {
	if disableEmojiFork {
		return nil
	}

	msg, err := b.channelMessage(ref)
	if err != nil {
		return fmt.Errorf("couldn't fetch full message to fork: %v", err)
	}

	//Check if the message already had a thread emoji and this is another one;
	//if so , don't start a new thread. It's weird to do this here, but we don't
	//have the reaction count on the message until fetching the message here.
	for _, reaction := range msg.Reactions {
		if reaction.Emoji == nil {
			continue
		}
		if reaction.Emoji.Name != FORK_THREAD_EMOJI {
			continue
		}
		if reaction.Count > 1 {
			fmt.Printf("Didn't fork message because there was already one " + FORK_THREAD_EMOJI + "\n")
			return nil
		}
	}

	//previousMessages will be most recent to least recent by default
	previousMessages, err := channelMessagesWithGuildID(b.session, ref.GuildID, ref.ChannelID, MESSAGES_TO_FETCH, ref.MessageID, "", "")
	if err != nil {
		return fmt.Errorf("couldn't fetch previous messages: %v", err)
	}

	var keptMessages []*discordgo.Message

	foundThreadStart := false

	for _, previousMessage := range previousMessages {
		if previousMessage.Type != discordgo.MessageTypeDefault && previousMessage.Type != discordgo.MessageTypeReply {
			continue
		}
		hasThreadStart := false
		hasThreadEnd := false
		for _, reaction := range previousMessage.Reactions {
			if reaction.Emoji.Name == FORK_THREAD_EMOJI {
				hasThreadEnd = true
			}
			if reaction.Emoji.Name == START_FORK_THREAD_EMOJI {
				hasThreadStart = true
			}
		}
		//If we find another thread end, then there must not be a thread start
		if hasThreadEnd {
			break
		}
		keptMessages = append(keptMessages, previousMessage)
		//We've added the last message we were supposed to fork
		if hasThreadStart {
			foundThreadStart = true
			break
		}
	}

	filteredMessages := []*discordgo.Message{msg}

	if foundThreadStart {
		//Only add the other messages before if we found a thread start
		filteredMessages = append(filteredMessages, keptMessages...)
	}

	//flip it so older messages are first, and newer messages are at end.
	for i, j := 0, len(filteredMessages)-1; i < j; i, j = i+1, j-1 {
		filteredMessages[i], filteredMessages[j] = filteredMessages[j], filteredMessages[i]
	}

	idf, err := b.getLiveIDFIndex(ref.GuildID)
	if err != nil {
		return fmt.Errorf("couldn't fetch live IDF: %v", err)
	}

	tfidf := idf.TFIDFForMessages(filteredMessages...)

	title := strings.Join(tfidf.AutoTopWords(6), "-")

	thread, err := b.createNewThreadInDefaultCategory(ref.GuildID, title)
	if err != nil {
		return fmt.Errorf("couldn't create thread: %v", err)
	}

	refs := make([]*discordgo.MessageReference, len(filteredMessages))

	for i, msg := range filteredMessages {
		refs[i] = msg.Reference()
	}

	if err := b.forkMessage(thread.ID, userID, refs...); err != nil {
		return fmt.Errorf("couldn't fork message: %v", err)
	}

	return nil
}

//session.channelMessages doesn't include GuildID. This sets it. See also bot.channelMessage()
func channelMessagesWithGuildID(session *discordgo.Session, guildID, channelID string, limit int, before, after, around string) ([]*discordgo.Message, error) {
	msgs, err := session.ChannelMessages(channelID, limit, before, after, around)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgs {
		msg.GuildID = guildID
	}
	return msgs, nil
}

func urlForMessage(message *discordgo.Message) string {
	return "https://discord.com/channels/" + message.GuildID + "/" + message.ChannelID + "/" + message.ID
}

//messageIsForkOf returns a non-zero-length string of the original message ID if
//the given message appears to be a forked message
func messageIsForkOf(message *discordgo.Message) *discordgo.MessageReference {
	if len(message.Embeds) == 0 {
		return nil
	}
	for _, embed := range message.Embeds {
		if embed.Title != FORKED_MESSAGE_LINK_TEXT {
			continue
		}
		urlPieces := strings.Split(embed.URL, "/")
		return &discordgo.MessageReference{
			ChannelID: urlPieces[len(urlPieces)-2],
			MessageID: urlPieces[len(urlPieces)-1],
		}
	}
	return nil
}

func messageEmbedAuthorForMessage(message *discordgo.Message) *discordgo.MessageEmbedAuthor {
	if message.Author == nil {
		return nil
	}
	return &discordgo.MessageEmbedAuthor{
		URL:     "https://discord.com/users/" + message.Author.ID,
		Name:    message.Author.Username,
		IconURL: message.Author.AvatarURL(""),
	}
}

//This is how we'll decide if a message with an embed is a forked message
const FORKED_MESSAGE_LINK_TEXT = "originally said:"

func createForkMessageEmbed(msg *discordgo.Message) *discordgo.MessageEmbed {
	var emojiDescriptions []string
	for _, reaction := range msg.Reactions {
		if reaction.Emoji.Name == FORK_THREAD_EMOJI {
			continue
		}
		if reaction.Emoji.Name == START_FORK_THREAD_EMOJI {
			continue
		}
		emojiDescriptions = append(emojiDescriptions, reaction.Emoji.MessageFormat()+" : "+strconv.Itoa(reaction.Count))
	}
	var fields []*discordgo.MessageEmbedField
	if len(emojiDescriptions) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Reactions",
			Value:  strings.Join(emojiDescriptions, "\t"),
			Inline: true,
		})
	}
	//Note: if you change this, also change messageIsFork to be able to detect
	//it!
	return &discordgo.MessageEmbed{
		Title:       FORKED_MESSAGE_LINK_TEXT,
		Description: msg.Content,
		Author:      messageEmbedAuthorForMessage(msg),
		URL:         urlForMessage(msg),
		Fields:      fields,
	}
}

//channelRef is a wrapper around session.ChannelMessage. The Discord API for
//some reason omits GuildID for messages fetched via ChannelMessage, but other
//processing assumes it exists. This method will fetch it but also stuff the
//GuildID in. See also channelMessagesWithGuildID
func (b *bot) channelMessage(ref *discordgo.MessageReference) (*discordgo.Message, error) {
	//OK, it has forks, we need to fetch the updated message.
	msg, err := b.session.ChannelMessage(ref.ChannelID, ref.MessageID)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch the raw updated message: %v", err)
	}
	//No idea why this is happening, but the Message comes back form
	//ChannelMessage with a zeroed guildID! Stuff it in for the sake of
	//downstream stuff...
	msg.GuildID = ref.GuildID
	return msg, nil
}

func (b *bot) updateForkedMessagesIfTheyExist(ref *discordgo.MessageReference) error {
	idf, err := b.getLiveIDFIndex(ref.GuildID)
	if err != nil {
		return fmt.Errorf("couldn't get idf in update forked messages if they exist: %v", err)
	}
	forks := idf.MessageForks(ref.ChannelID, ref.MessageID)
	if len(forks) == 0 {
		return nil
	}
	//OK, it has forks, we need to fetch the updated message.
	sourceMessage, err := b.channelMessage(ref)
	if err != nil {
		return fmt.Errorf("couldn't fetch the raw updated message: %v", err)
	}

	if err := b.updateForkedMessages(sourceMessage); err != nil {
		return fmt.Errorf("couldn't update forked messages: %v", err)
	}
	return nil
}

const NOT_REST_ERROR_CODE = -1

func restErrorCode(err error) int {
	if e, ok := err.(*discordgo.RESTError); ok {
		if e.Message != nil {
			return e.Message.Code
		}
	}
	return NOT_REST_ERROR_CODE
}

//updates the forked messages that are forks of sourceMessage, if there are any
func (b *bot) updateForkedMessages(sourceMessage *discordgo.Message) error {
	idf, err := b.getLiveIDFIndex(sourceMessage.GuildID)
	if err != nil {
		return fmt.Errorf("couldn't get idf in message update: %v", err)
	}
	forks := idf.MessageForks(sourceMessage.ChannelID, sourceMessage.ID)
	if len(forks) == 0 {
		return nil
	}
	embed := createForkMessageEmbed(sourceMessage)
	for _, fork := range forks {
		if _, err := b.session.ChannelMessageEditEmbed(fork.ChannelID, fork.MessageID, embed); err != nil {
			if restErrorCode(err) == discordgo.ErrCodeUnknownMessage {
				//Perhaps the forked message that we saw at some point
				//has been deleted. That's fine, just skip it!
				continue
			}
			return fmt.Errorf("couldn't update forked message for source %v and target %v: %v", sourceMessage.ID, fork.MessageID, err)
		}
		fmt.Printf("updated message %v to %v because the message it was forked from (%v) changed\n", fork.MessageID, sourceMessage.Content, sourceMessage.ID)
	}
	return nil
}

func (b *bot) forkMessage(targetChannelID string, userID string, sourceRefs ...*discordgo.MessageReference) error {

	if len(sourceRefs) == 0 {
		return nil
	}

	firstRef := sourceRefs[0]

	if _, err := b.session.ChannelMessageSend(targetChannelID, "Forking messages from <#"+firstRef.ChannelID+"> because of a "+FORK_THREAD_EMOJI+" reaction by <@"+userID+">. If you don't like the auto-generated title, you can change it."); err != nil {
		return fmt.Errorf("couldn't post initial thread messagae: %v", err)
	}

	for i, sourceRef := range sourceRefs {
		//TODO: it is expensive to fetch each of these individually, especially
		//since likely upstream we have them already.
		msg, err := b.channelMessage(sourceRef)
		if err != nil {
			return fmt.Errorf("couldn't fetch message %v: %v", i, err)
		}

		embed := createForkMessageEmbed(msg)

		if _, err := b.session.ChannelMessageSendEmbed(targetChannelID, embed); err != nil {
			return fmt.Errorf("couldn't send message %v: %v", i, err)
		}
	}

	lastSourceRef := sourceRefs[len(sourceRefs)-1]

	var message string

	if len(sourceRefs) == 1 {
		message = "Forked 1 message to <#" + targetChannelID + ">. If you would have marked an earlier message with " + START_FORK_THREAD_EMOJI + " then all of the messages between the two emojis would have been forked."
	} else {
		message = "Forked " + strconv.Itoa(len(sourceRefs)) + " messages to <#" + targetChannelID + ">."
	}

	data := &discordgo.MessageSend{
		Content:   message,
		Reference: lastSourceRef,
	}

	if _, err := b.session.ChannelMessageSendComplex(lastSourceRef.ChannelID, data); err != nil {
		return fmt.Errorf("couldn't post read out message for fork: %v", err)
	}

	return nil

}

func (b *bot) interactionCreate(s *discordgo.Session, event *discordgo.InteractionCreate) {
	//NOTE: all handlers must use s.InteractionRespond or the user will see an error.
	switch event.Interaction.Data.Name {
	case ARCHIVE_COMMAND_NAME:
		b.archiveThreadInteraction(s, event)
	case SUGGEST_THREAD_NAME_COMMAND_NAME:
		b.suggestThreadNameInteraction(s, event)
	default:
		fmt.Println("Unknown interaction name: " + event.Interaction.Data.Name)
	}
}

func (b *bot) noteMessageIfFork(msg *discordgo.Message) error {
	forkedFrom := messageIsForkOf(msg)
	if forkedFrom == nil {
		return nil
	}
	fmt.Printf("Indexing %v which appears to be a fork\n", msg.ID)

	idf, err := b.getLiveIDFIndex(msg.GuildID)
	if err != nil {
		return fmt.Errorf("couldn't fetch idf: %v", err)
	}
	idf.NoteForkedMessage(forkedFrom, msg.Reference())
	idf.RequestPeristence()
	return nil
}

func (b *bot) scheduleRebuildIDFCache() {
	if b.rebuildIDFTimer != nil {
		b.rebuildIDFTimer.Stop()
	}
	//Set up timer to rebuild IDF caches automatically. We run way more often
	//than the actual interval; if we run too early, it's OK, we'll just load
	//the cache from disk without rebuilding. Running so often means that we
	//avoid lots of timing issues where we run _just_ before the cache expires,
	//meaning we would have waited roughly 2x the expiration interval.
	b.rebuildIDFTimer = time.AfterFunc(REBUILD_IDF_INTERVAL/16, b.rebuildIDFCaches)
}

func (b *bot) getLiveIDFIndex(guildID string) (*IDFIndex, error) {
	result := b.indexes[guildID]
	if result != nil {
		return result, nil
	}
	result, err := IDFIndexForGuild(guildID, b.session)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch live IDF index: %v", err)
	}
	b.indexes[guildID] = result
	return result, nil
}

func (b *bot) rebuildIDFCaches() {
	fmt.Printf("Checking if IDF caches need rebuilding\n")

	for guildID := range b.infos {
		if !IDFIndexForGuildNeedsRebuilding(guildID) && b.indexes[guildID] != nil {
			continue
		}
		idf, err := IDFIndexForGuild(guildID, b.session)
		if err != nil {
			fmt.Printf("couldn't recreate guild idf for guild %v: %v\n", guildID, err)
		}
		b.indexes[guildID] = idf
	}
	b.scheduleRebuildIDFCache()
}

func (b *bot) suggestThreadNameInteraction(s *discordgo.Session, event *discordgo.InteractionCreate) {

	//We have to respond to the message within 3 seconds, and it might take
	//longer to fetch all messages, so we have to do a deferred channel message.

	s.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	idf, err := IDFIndexForGuild(event.GuildID, s)

	if err != nil {
		s.InteractionResponseEdit(s.State.User.ID, event.Interaction, &discordgo.WebhookEdit{
			Content: "*Error* Couldn't generate IDF index for channel " + err.Error(),
		})
		return
	}

	channel, err := b.session.State.Channel(event.ChannelID)

	if err != nil {
		s.InteractionResponseEdit(s.State.User.ID, event.Interaction, &discordgo.WebhookEdit{
			Content: "*Error* Couldn't get channel: " + err.Error(),
		})
		return
	}

	channelMessages, err := FetchAllMessagesForChannel(s, channel)

	if err != nil {
		s.InteractionResponseEdit(s.State.User.ID, event.Interaction, &discordgo.WebhookEdit{
			Content: "*Error* Couldn't fetch channel messages for channel " + err.Error(),
		})
		return
	}

	tfidf := idf.TFIDFForMessages(channelMessages...)
	topWords := tfidf.AutoTopWords(6)

	s.InteractionResponseEdit(s.State.User.ID, event.Interaction, &discordgo.WebhookEdit{
		Content: "Suggested thread title: " + strings.Join(topWords, "-"),
	})

}

func (b *bot) archiveThreadInteraction(s *discordgo.Session, event *discordgo.InteractionCreate) {

	channel, err := b.session.State.Channel(event.ChannelID)
	if err != nil {
		//TODO: respond to the interaction in the canonical way so it shows up in user's UI
		fmt.Printf("Couldn't fetch channel %v: %v\n", event.ChannelID, err)
		return
	}
	message := "Couldn't archive: "
	gi := b.getThreadGroupInfoForThread(channel)
	if gi != nil {
		if err := gi.archiveThread(b.controller, s, channel); err != nil {
			message += err.Error()
			fmt.Println(message)
		} else {
			message = "Archived thread!"
		}
	} else {
		message += "This channel is not a thread!"
	}

	s.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionApplicationCommandResponseData{
			Content: message,
		},
	})
}

func (b *bot) setGuildNeedsInfoRegeneration(guildID string) {
	b.infoMutex.Lock()
	delete(b.infos, guildID)
	b.infoMutex.Unlock()
}

func (b *bot) getInfos(guildID string) categoryMap {
	b.infoMutex.RLock()
	currentInfos := b.infos[guildID]
	b.infoMutex.RUnlock()
	if currentInfos == nil {
		b.rebuildCategoryMap(guildID, false)
	}
	b.infoMutex.RLock()
	currentInfos = b.infos[guildID]
	b.infoMutex.RUnlock()
	return currentInfos
}

//returns nil if not a thraed
func (b *bot) getThreadGroupInfoForThread(channel *discordgo.Channel) *threadGroupInfo {
	guildInfos := b.getInfos(channel.GuildID)
	if guildInfos == nil {
		//Must be a message from a server without a Threads category
		return nil
	}
	for _, group := range guildInfos {
		if channel.ParentID == group.threadCategoryID {
			//A message outside of Threads category
			return group
		}
	}
	//Didn't match any of the infos
	return nil
}

func (b *bot) isThread(channel *discordgo.Channel) bool {
	return b.getThreadGroupInfoForThread(channel) != nil
}

type categoryStruct struct {
	threadGroup       *discordgo.Channel
	archiveCategories byArchiveIndex
}

func createCategoryMap(guild *discordgo.Guild, alert bool) (infos categoryMap) {
	categories := make(map[string]*categoryStruct)

	for _, channel := range guild.Channels {
		if channel.Type != discordgo.ChannelTypeGuildCategory {
			continue
		}
		// THREAD_ARCHIVE_CATEGORY_NAME is a superset of THREAD_CATEGORY_NAME so check for that first
		if strings.Contains(channel.Name, THREAD_ARCHIVE_CATEGORY_NAME) {
			name := strings.TrimSpace(strings.Split(channel.Name, THREAD_ARCHIVE_CATEGORY_NAME)[0])
			category := categories[name]
			if category == nil {
				category = &categoryStruct{}
				categories[name] = category
			}
			category.archiveCategories = append(category.archiveCategories, channel)
			continue
		}
		if strings.Contains(channel.Name, THREAD_CATEGORY_NAME) {
			name := strings.TrimSpace(strings.Split(channel.Name, THREAD_CATEGORY_NAME)[0])
			category := categories[name]
			if category == nil {
				category = &categoryStruct{}
				categories[name] = category
			}
			category.threadGroup = channel
		}

	}

	infos = make(categoryMap)

	for name, category := range categories {

		sort.Sort(category.archiveCategories)

		var archiveIDs []string
		var activeArchiveCategoryID string
		var nextArchiveCategoryIndex int
		for i, channel := range category.archiveCategories {
			if i == 0 {
				nextArchiveCategoryIndex = indexForThreadArchive(channel) + 1
				// It can only be active if there's at least one thread slot
				if numThreadsInCategory(guild, channel) < MAX_CATEGORY_CHANNELS {
					activeArchiveCategoryID = channel.ID
				}
			}
			archiveIDs = append(archiveIDs, channel.ID)
		}

		printName := name
		if printName == "" {
			printName = "''"
		}

		if alert {
			fmt.Println("Found " + THREAD_CATEGORY_NAME + " category named " + printName + " in guild " + nameForGuild(guild) + " with " + strconv.Itoa(len(archiveIDs)) + " archive categories")
		}

		info := &threadGroupInfo{
			name:                     name,
			threadCategoryID:         category.threadGroup.ID,
			activeArchiveCategoryID:  activeArchiveCategoryID,
			archiveCategoryIDs:       archiveIDs,
			nextArchiveCategoryIndex: nextArchiveCategoryIndex,
		}

		infos[category.threadGroup.ID] = info
	}

	if len(categories) == 0 {
		if alert {
			fmt.Println(guild.Name + " (ID " + guild.ID + ") joined but didn't have a category named " + THREAD_CATEGORY_NAME)
		}
	}
	return
}

// rebuildCategoryMap should be called any time the categories in the given guild
// may have changed, e.g. a channel was created, updated, or the guild was seen
// for the first time. if alert is true, then it will print formatting if it
// errors.
func (b *bot) rebuildCategoryMap(guildID string, alert bool) {
	guild, err := b.session.State.Guild(guildID)

	if err != nil {
		fmt.Println("Couldn't get guild " + guildID)
		return
	}

	infos := createCategoryMap(guild, alert)

	b.infoMutex.Lock()
	b.infos[guild.ID] = infos
	b.infoMutex.Unlock()
}

func (b *bot) createNewThreadInDefaultCategory(guildID string, threadName string) (*discordgo.Channel, error) {
	infos := b.getInfos(guildID)
	var categoryID string
	//find at least one categoryID, defaulting to the one that is default ""
	for id, info := range infos {
		categoryID = id
		if info.name == "" {
			break
		}
	}

	if categoryID == "" {
		return nil, fmt.Errorf("thread is no threads category to create a thread in")
	}

	return b.controller.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:     threadName,
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: categoryID,
	})
}

func numThreadsInCategory(guild *discordgo.Guild, category *discordgo.Channel) int {
	threads := threadsInCategory(guild, category)
	if threads == nil {
		return -1
	}
	return len(threads)
}

func threadsInCategory(guild *discordgo.Guild, category *discordgo.Channel) []*discordgo.Channel {
	var result byDiscordOrder
	for _, channel := range guild.Channels {
		if channel.ParentID == category.ID {
			result = append(result, channel)
		}
	}
	sort.Sort(result)
	return result
}

// Moves this thread to position 0, sliding everything else down, but maintaining
// their order.
func (b *bot) moveThreadToTopOfCategory(thread *discordgo.Channel) error {

	guild, err := b.session.State.Guild(thread.GuildID)
	if err != nil {
		return fmt.Errorf("couldn't fetch guild: %w", err)
	}
	var threads byDiscordOrder
	// the thread we want to move to the head, but refreshed
	var headThread *discordgo.Channel
	for _, channel := range guild.Channels {
		if channel.ParentID == thread.ParentID {
			if channel.ID == thread.ID {
				headThread = channel
			} else {
				threads = append(threads, channel)
			}
		}
	}

	// The order we come across them in has nothing to do with their actual order...
	sort.Sort(threads)

	if headThread == nil {
		return fmt.Errorf("didn't find the target thread unexpectedly")
	}

	threads = append([]*discordgo.Channel{headThread}, threads...)

	for i, channel := range threads {
		channel.Position = i
	}

	if err := b.controller.GuildChannelsReorder(guild.ID, threads); err != nil {
		return fmt.Errorf("couldn't reorder channels: %w", err)
	}

	return nil
}

func (b *bot) moveThreadToTopOfThreads(thread *discordgo.Channel) error {

	fmt.Println("Popping thread " + nameForThread(thread) + " to top because it received a new message")

	// TODO: conceptually we should move this to the given category if it's not in it yet.
	return b.moveThreadToTopOfCategory(thread)

}

func (b byArchiveIndex) Len() int {
	return len(b)
}

func (b byArchiveIndex) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byArchiveIndex) Less(i, j int) bool {
	left := b[i]
	right := b[j]
	// Sort so the ones with the higher index come first
	return indexForThreadArchive(left) > indexForThreadArchive(right)
}

func (b byDiscordOrder) Len() int {
	return len(b)
}

func (b byDiscordOrder) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byDiscordOrder) Less(i, j int) bool {
	left := b[i]
	right := b[j]
	// Notionally this should be similar logic to https://github.com/Rapptz/discord.py/issues/2392#issuecomment-707455919
	// For now simply sorting by position index is fine

	return left.Position < right.Position
}

func (g *threadGroupInfo) archiveThreadsIfNecessary(controller Controller, session *discordgo.Session) error {
	category, err := session.State.Channel(g.threadCategoryID)
	if err != nil {
		return fmt.Errorf("archiveThreadsIfNecessary couldn't find category: %w", err)
	}
	guild, err := session.State.Guild(category.GuildID)
	if err != nil {
		return fmt.Errorf("archiveThreadsIfNecessary couldn't find guild: %w", err)
	}

	threads := threadsInCategory(guild, category)

	if len(threads) <= maxActiveThreads {
		// Not necessary to remove any
		return nil
	}

	extraCount := len(threads) - maxActiveThreads

	for i := 0; i < extraCount; i++ {
		thread := threads[len(threads)-1-i]
		if err := g.archiveThread(controller, session, thread); err != nil {
			return fmt.Errorf("couldn't archive thread %v: %w", i, err)
		}
	}

	return nil
}

func (g *threadGroupInfo) archiveThread(controller Controller, session *discordgo.Session, thread *discordgo.Channel) error {
	fmt.Println("Archiving thread " + nameForThread(thread) + " to because it no longer fits")
	var activeArchiveCategoryID = g.activeArchiveCategoryID
	if activeArchiveCategoryID == "" {
		// Need to create an archive category to put into

		guild, err := session.State.Guild(thread.GuildID)
		if err != nil {
			return fmt.Errorf("couldn't get guild: %w", err)
		}

		mainCategory, err := session.State.Channel(g.threadCategoryID)
		if err != nil {
			return fmt.Errorf("couldn't get main category: %w", err)
		}

		var everyoneRoleID string
		for _, role := range guild.Roles {
			if strings.Contains(role.Name, EVERYONE_ROLE_NAME) {
				everyoneRoleID = role.ID
			}
		}

		if everyoneRoleID == "" {
			return fmt.Errorf("couldn't find role @everyone")
		}

		var name string
		if g.name != "" {
			name = g.name + " "
		}

		name += THREAD_ARCHIVE_CATEGORY_NAME + " " + strconv.Itoa(g.nextArchiveCategoryIndex)

		extendedPermissions := []*discordgo.PermissionOverwrite{
			{
				ID:   everyoneRoleID,
				Type: discordgo.PermissionOverwriteTypeRole,
				Deny: discordgo.PermissionSendMessages,
			},
		}
		// Copy over the main categorie's permissions
		extendedPermissions = append(extendedPermissions, mainCategory.PermissionOverwrites...)

		archiveCategory, err := controller.GuildChannelCreateComplex(thread.GuildID, discordgo.GuildChannelCreateData{
			Name:                 name,
			Type:                 discordgo.ChannelTypeGuildCategory,
			PermissionOverwrites: extendedPermissions,
		})

		if err != nil {
			return fmt.Errorf("archiveThreadCreate failed: %w", err)
		}

		activeArchiveCategoryID = archiveCategory.ID
	}

	archiveCategory, err := session.State.Channel(activeArchiveCategoryID)
	if err != nil {
		return fmt.Errorf("couldn't fetch archiveCategory: %w", err)
	}

	_, err = controller.ChannelEditComplex(thread.ID, &discordgo.ChannelEdit{
		// This is a generally reasonable default, especially because by default
		// there will very often only be one.
		Position: 0,
		ParentID: activeArchiveCategoryID,
		// Set the same permission overwrites so it will be synced
		PermissionOverwrites: archiveCategory.PermissionOverwrites,
	})

	if err != nil {
		return fmt.Errorf("couldn't move categories: %w", err)
	}

	// TODO: we really should make sure the thread is at the top of the archive.
	// But b.moveThreadToTopOfCategory won't work naively because at this point
	// we haven't yet received the channelUpdate message (I think). Currently the
	// behavior works OK if only one thread is being archived.

	return nil
}

//Called before the program exits when the bot should clean up, persist state, etc.
func (b *bot) Close() {
	for _, index := range b.indexes {
		if err := index.Persist(); err != nil {
			fmt.Printf("Couln't persist IDF %v: %v\n", index.guildID, err)
			//Continue on and try to save the other ones too
		}
	}
}

func indexForThreadArchive(channel *discordgo.Channel) int {
	pieces := strings.Split(channel.Name, THREAD_ARCHIVE_CATEGORY_NAME)
	if len(pieces) == 1 {
		return -1
	}
	intStr := strings.TrimSpace(pieces[1])
	result, err := strconv.Atoi(intStr)
	if err != nil {
		fmt.Println("Couldn't convert string: " + intStr + ": " + err.Error())
		return -1
	}
	return result
}

func nameForGuild(guild *discordgo.Guild) string {
	return guild.Name + " (" + guild.ID + ")"
}

func nameForThread(thread *discordgo.Channel) string {
	return thread.Name + " (" + thread.ID + ")"
}
