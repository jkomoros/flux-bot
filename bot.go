package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type bot struct {
	session *discordgo.Session
	//guildID -> threadCategoryChannelID -> info
	infos map[string]map[string]*threadGroupInfo
}

type threadGroupInfo struct {
	b                        *bot
	name                     string
	threadCategoryID         string
	nextArchiveCategoryIndex int
	activeArchiveCategoryID  string
	archiveCategoryIDs       []string
}

type byArchiveIndex []*discordgo.Channel

//Sort in a similar way as the main discord client
type byDiscordOrder []*discordgo.Channel

func newBot(s *discordgo.Session) *bot {
	result := &bot{
		session: s,
		infos:   make(map[string]map[string]*threadGroupInfo),
	}
	s.AddHandler(result.ready)
	s.AddHandler(result.guildCreate)
	s.AddHandler(result.messageCreate)
	s.AddHandler(result.channelCreate)
	s.AddHandler(result.channelUpdate)
	return result
}

// This function will be called (due to AddHandler above) when the bot receives
// the "ready" event from Discord.
func (b *bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	//GuildInfo isn't populated yet.
	fmt.Println("Ready and waiting!")
}

//This will be called after the bot starts up for each guild it's added to
func (b *bot) guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	b.rebuildCategoryMap(event.Guild.ID, true)
	guildInfos := b.getInfos(event.Guild.ID)
	if guildInfos == nil {
		fmt.Printf("Couldn't find guild with ID %v", event.Guild.ID)
	}
	for _, group := range guildInfos {
		if err := group.archiveThreadsIfNecessary(); err != nil {
			fmt.Printf("Couldn't archive extra threads on boot: %v", err)
		}
	}
}

func (b *bot) messageCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
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

func (b *bot) channelCreate(s *discordgo.Session, event *discordgo.ChannelCreate) {
	b.rebuildCategoryMap(event.GuildID, true)

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
		if err := group.archiveThreadsIfNecessary(); err != nil {
			fmt.Printf("Couldn't archive threads if necessary: %v", err)
		}
	}
}

func (b *bot) channelUpdate(s *discordgo.Session, event *discordgo.ChannelUpdate) {
	//channelUpdate happens a LOT, e.g. every time we reorder a channel, every
	//single channel whose index changed will get called one at a time. So don't log.
	b.rebuildCategoryMap(event.GuildID, false)
}

func (b *bot) getInfos(guildID string) map[string]*threadGroupInfo {
	return b.infos[guildID]
}

func (b *bot) isThread(channel *discordgo.Channel) bool {
	guildInfos := b.getInfos(channel.GuildID)
	if guildInfos == nil {
		//Must be a message from a server without a Threads category
		return false
	}
	for _, group := range guildInfos {
		if channel.ParentID == group.threadCategoryID {
			//A message outside of Threads category
			return true
		}
	}
	//Didn't match any of the infos
	return false
}

type categoryStruct struct {
	threadGroup       *discordgo.Channel
	archiveCategories byArchiveIndex
}

//rebuildCategoryMap should be called any time the categories in the given guild
//may have changed, e.g. a channel was created, updated, or the guild was seen
//for the first time. if alert is true, then it will print formatting if it
//errors.
func (b *bot) rebuildCategoryMap(guildID string, alert bool) {

	guild, err := b.session.State.Guild(guildID)

	if err != nil {
		fmt.Println("Couldn't get guild " + guildID)
		return
	}

	categories := make(map[string]*categoryStruct)

	for _, channel := range guild.Channels {
		if channel.Type != discordgo.ChannelTypeGuildCategory {
			continue
		}
		//THREAD_ARCHIVE_CATEGORY_NAME is a superset of THREAD_CATEGORY_NAME so check for that first
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

	b.infos[guild.ID] = make(map[string]*threadGroupInfo)

	for name, category := range categories {

		sort.Sort(category.archiveCategories)

		var archiveIDs []string
		var activeArchiveCategoryID string
		var nextArchiveCategoryIndex int
		for i, channel := range category.archiveCategories {
			if i == 0 {
				nextArchiveCategoryIndex = indexForThreadArchive(channel) + 1
				//It can only be active if there's at least one thread slot
				if b.numThreadsInCategory(channel) < MAX_CATEGORY_CHANNELS {
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
			b:                        b,
			name:                     name,
			threadCategoryID:         category.threadGroup.ID,
			activeArchiveCategoryID:  activeArchiveCategoryID,
			archiveCategoryIDs:       archiveIDs,
			nextArchiveCategoryIndex: nextArchiveCategoryIndex,
		}

		b.infos[guild.ID][category.threadGroup.ID] = info
	}

	if len(categories) == 0 {
		if alert {
			fmt.Println(guild.Name + " (ID " + guild.ID + ") joined but didn't have a category named " + THREAD_CATEGORY_NAME)
		}
	}

}

func (b *bot) numThreadsInCategory(category *discordgo.Channel) int {
	threads := b.threadsInCategory(category)
	if threads == nil {
		return -1
	}
	return len(threads)
}

func (b *bot) threadsInCategory(category *discordgo.Channel) []*discordgo.Channel {
	guild, err := b.session.State.Guild(category.GuildID)
	if err != nil {
		return nil
	}
	var result byDiscordOrder
	for _, channel := range guild.Channels {
		if channel.ParentID == category.ID {
			result = append(result, channel)
		}
	}
	sort.Sort(result)
	return result
}

//Moves this thread to position 0, sliding everything else down, but maintaining
//their order.
func (b *bot) moveThreadToTopOfCategory(thread *discordgo.Channel) error {

	guild, err := b.session.State.Guild(thread.GuildID)
	if err != nil {
		return fmt.Errorf("couldn't fetch guild: %w", err)
	}
	var threads byDiscordOrder
	//the thread we want to move to the head, but refreshed
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

	//The order we come across them in has nothing to do with their actual order...
	sort.Sort(threads)

	if headThread == nil {
		return fmt.Errorf("didn't find the target thread unexpectedly")
	}

	threads = append([]*discordgo.Channel{headThread}, threads...)

	for i, channel := range threads {
		channel.Position = i
	}

	if err := b.session.GuildChannelsReorder(guild.ID, threads); err != nil {
		return fmt.Errorf("couldn't reorder channels: %w", err)
	}

	return nil
}

func (b *bot) moveThreadToTopOfThreads(thread *discordgo.Channel) error {

	fmt.Println("Popping thread " + nameForThread(thread) + " to top because it received a new message")

	//TODO: conceptually we should move this to the given category if it's not in it yet.
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
	//Sort so the ones with the higher index come first
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
	//Notionally this should be similar logic to https://github.com/Rapptz/discord.py/issues/2392#issuecomment-707455919
	//For now simply sorting by position index is fine

	return left.Position < right.Position
}

func (g *threadGroupInfo) archiveThreadsIfNecessary() error {
	category, err := g.b.session.State.Channel(g.threadCategoryID)
	if err != nil {
		return fmt.Errorf("archiveThreadsIfNecessary couldn't find category: %w", err)
	}
	threads := g.b.threadsInCategory(category)

	if len(threads) <= maxActiveThreads {
		//Not necessary to remove any
		return nil
	}

	extraCount := len(threads) - maxActiveThreads

	for i := 0; i < extraCount; i++ {
		thread := threads[len(threads)-1-i]
		if err := g.archiveThread(thread); err != nil {
			return fmt.Errorf("couldn't archive thread %v: %w", i, err)
		}
	}

	return nil
}

func (g *threadGroupInfo) archiveThread(thread *discordgo.Channel) error {
	fmt.Println("Archiving thread " + nameForThread(thread) + " to because it no longer fits")
	var activeArchiveCategoryID = g.activeArchiveCategoryID
	if activeArchiveCategoryID == "" {
		//Need to create an archive category to put into

		guild, err := g.b.session.State.Guild(thread.GuildID)
		if err != nil {
			return fmt.Errorf("couldn't get guild: %w", err)
		}

		mainCategory, err := g.b.session.State.Channel(g.threadCategoryID)
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
		//Copy over the main categorie's permissions
		extendedPermissions = append(extendedPermissions, mainCategory.PermissionOverwrites...)

		archiveCategory, err := g.b.session.GuildChannelCreateComplex(thread.GuildID, discordgo.GuildChannelCreateData{
			Name:                 name,
			Type:                 discordgo.ChannelTypeGuildCategory,
			PermissionOverwrites: extendedPermissions,
		})

		if err != nil {
			return fmt.Errorf("archiveThreadCreate failed: %w", err)
		}

		activeArchiveCategoryID = archiveCategory.ID
	}

	archiveCategory, err := g.b.session.State.Channel(activeArchiveCategoryID)
	if err != nil {
		return fmt.Errorf("couldn't fetch archiveCategory: %w", err)
	}

	_, err = g.b.session.ChannelEditComplex(thread.ID, &discordgo.ChannelEdit{
		//This is a generally reasonable default, especially because by default
		//there will very often only be one.
		Position: 0,
		ParentID: activeArchiveCategoryID,
		//Set the same permission overwrites so it will be synced
		PermissionOverwrites: archiveCategory.PermissionOverwrites,
	})

	if err != nil {
		return fmt.Errorf("couldn't move categories: %w", err)
	}

	//TODO: we really should make sure the thread is at the top of the archive.
	//But b.moveThreadToTopOfCategory won't work naively because at this point
	//we haven't yet received the channelUpdate message (I think). Currently the
	//behavior works OK if only one thread is being archived.

	return nil
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
