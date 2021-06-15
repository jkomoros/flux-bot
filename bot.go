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

//A hard coded temporary constant for a thread to fork to. To be removed once
//better forking targeting specific channels exists!! Only valid in the Komorama
//test server!
const TEMP_FORK_CHANNEL = "839640072613396491"

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
	s.AddHandler(result.channelCreate)
	s.AddHandler(result.channelUpdate)
	s.AddHandler(result.messageReactionAdd)
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
		fmt.Printf("Couldn't find guild with ID %v", event.Guild.ID)
	}
	for _, group := range guildInfos {
		if err := group.archiveThreadsIfNecessary(b.controller, b.session); err != nil {
			fmt.Printf("Couldn't archive extra threads on boot: %v", err)
		}
	}
	//ensure that an IDF index exists, or build it now so we'll have it if we need it
	idf, err := IDFIndexForGuild(event.Guild.ID, s)
	if err != nil {
		fmt.Printf("couldn't fetch idf for guild %v: %v", event.Guild.ID, err)
	}
	b.indexes[event.Guild.ID] = idf
}

// discordgo callback: called after the when new message is posted.
func (b *bot) messageCreate(s *discordgo.Session, event *discordgo.MessageCreate) {

	if err := b.noteMessageIfFork(event.Message); err != nil {
		fmt.Printf("couldn't note forked message: %v", err)
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
		fmt.Printf("message received in a thread but couldn't move it: %v", err)
	}
}

// discordgo callback: called after the when a message is edited
func (b *bot) messageUpdate(s *discordgo.Session, event *discordgo.MessageUpdate) {
	if err := b.updateForkedMessages(event.Message); err != nil {
		fmt.Printf("couldn't update forked messages if any existed: %v", err)
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
		fmt.Printf("message received in a thread but couldn't move it: %v", err)
	}
	guildInfos := b.getInfos(event.GuildID)
	if guildInfos == nil {
		fmt.Println("Couldnt get guild info to archive if necessary")
		return
	}
	for _, group := range guildInfos {
		if err := group.archiveThreadsIfNecessary(b.controller, b.session); err != nil {
			fmt.Printf("Couldn't archive threads if necessary: %v", err)
		}
	}
}

// discordgo callback: channelUpdate happens a LOT, e.g. every time we reorder a channel, every
// single channel whose index changed will get called one at a time.
func (b *bot) channelUpdate(s *discordgo.Session, event *discordgo.ChannelUpdate) {
	b.setGuildNeedsInfoRegeneration(event.GuildID)
}

func (b *bot) messageReactionAdd(s *discordgo.Session, event *discordgo.MessageReactionAdd) {
	switch event.Emoji.Name {
	case FORK_THREAD_EMOJI:
		b.forkThreadViaEmoji(event.ChannelID, event.MessageID)
	}
}

func (b *bot) forkThreadViaEmoji(channelID, messageID string) {
	if disableEmojiFork {
		return
	}
	//TODO: don't use a temp_fork_channel
	if err := b.forkMessage(channelID, messageID, TEMP_FORK_CHANNEL); err != nil {
		fmt.Printf("Couldn't fork message: %v", err)
	}
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
		emojiDescriptions = append(emojiDescriptions, reaction.Emoji.Name+" : "+strconv.Itoa(reaction.Count))
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
			return fmt.Errorf("couldn't update forked message for source %v and target %v: %v", sourceMessage.ID, fork.MessageID, err)
		}
		fmt.Printf("updated message %v to %v because the message it was forked from (%v) changed", fork.MessageID, sourceMessage.Content, sourceMessage.ID)
	}
	return nil
}

func (b *bot) forkMessage(sourceChannelID, sourceMessageID, targetChannelID string) error {
	msg, err := b.session.ChannelMessage(sourceChannelID, sourceMessageID)
	if err != nil {
		return fmt.Errorf("couldn't fetch message: %v", err)
	}

	embed := createForkMessageEmbed(msg)

	if _, err := b.session.ChannelMessageSendEmbed(targetChannelID, embed); err != nil {
		return fmt.Errorf("couldn't send message: %v", err)
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
			fmt.Printf("couldn't recreate guild idf for guild %v: %v", guildID, err)
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
		fmt.Printf("Couldn't fetch channel %v: %v", event.ChannelID, err)
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
			fmt.Printf("Couln't persist IDF %v: %v", index.guildID, err)
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
