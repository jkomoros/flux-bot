package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type bot struct {
	session    *discordgo.Session
	guildInfos map[string]*guildInfo
}

type guildInfo struct {
	threadCategoryID string
}

func newBot(s *discordgo.Session) *bot {
	result := &bot{
		session:    s,
		guildInfos: make(map[string]*guildInfo),
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
}

func (b *bot) channelUpdate(s *discordgo.Session, event *discordgo.ChannelUpdate) {
	//channelUpdate happens a LOT, e.g. every time we reorder a channel, every
	//single channel whose index changed will get called one at a time. So don't log.
	b.rebuildCategoryMap(event.GuildID, false)
}

func (b *bot) isThread(channel *discordgo.Channel) bool {
	gi := b.guildInfos[channel.GuildID]
	if gi == nil {
		//Must be a message from a server without a Threads category
		return false
	}
	if channel.ParentID != gi.threadCategoryID {
		//A message outside of Threads category
		return false
	}
	return true
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

	var threadsCategory *discordgo.Channel

	for _, channel := range guild.Channels {
		if channel.Type != discordgo.ChannelTypeGuildCategory {
			continue
		}
		if channel.Name == THREAD_CATEGORY_NAME {
			threadsCategory = channel
			continue
		}
	}

	if threadsCategory == nil {
		if alert {
			fmt.Println(guild.Name + " (ID " + guild.ID + ") joined but didn't have a category named " + THREAD_CATEGORY_NAME)
		}
		return
	}

	if alert {
		fmt.Println("Found " + THREAD_CATEGORY_NAME + " category in guild " + nameForGuild(guild))
	}

	info := &guildInfo{
		threadCategoryID: threadsCategory.ID,
	}

	b.guildInfos[guild.ID] = info

}

func (b *bot) moveThreadToTopOfThreads(thread *discordgo.Channel) error {

	fmt.Println("Popping thread " + nameForThread(thread) + " to top because it received a new message")

	guild, err := b.session.State.Guild(thread.GuildID)
	if err != nil {
		return fmt.Errorf("couldn't fetch guild: %w", err)
	}
	gi := b.guildInfos[guild.ID]
	if gi == nil {
		return fmt.Errorf("guild didn't have threads")
	}
	var threads []*discordgo.Channel
	//the thread we want to move to the head, but refreshed
	var headThread *discordgo.Channel
	for _, channel := range guild.Channels {
		if channel.ParentID == gi.threadCategoryID {
			if channel.ID == thread.ID {
				headThread = channel
			} else {
				threads = append(threads, channel)
			}
		}
	}

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

func nameForGuild(guild *discordgo.Guild) string {
	return guild.Name + " (" + guild.ID + ")"
}

func nameForThread(thread *discordgo.Channel) string {
	return thread.Name + " (" + thread.ID + ")"
}