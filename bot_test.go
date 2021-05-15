package main

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

const (
	fakeToken string = "fake-value"
)

func newSessionStub() *discordgo.Session {
	session, _ := discordgo.New(fakeToken)
	return session
}

func newReadyEvent() *discordgo.Ready {
	return nil
}

func newGuildCreate(guild *discordgo.Guild) *discordgo.GuildCreate {
	return &discordgo.GuildCreate{
		Guild: guild,
	}
}

func TestReady(t *testing.T) {
	session := newSessionStub()
	bot := newBot(session)
	bot.ready(session, newReadyEvent())
}

func TestGuildCreateSimple(t *testing.T) {
	session := newSessionStub()
	bot := newBot(session)
	guild := &discordgo.Guild{
		ID: "guild-1",
	}
	// GuildID is necessary in archiveThreadsIfNecessary
	// TODO: maybe find a way to not need it
	guild.Channels = []*discordgo.Channel{
		{
			ID:      "thread-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_ARCHIVE_CATEGORY_NAME + " 72",
			GuildID: "guild-1",
		},
		{
			ID:      "thread-archive-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_CATEGORY_NAME,
			GuildID: "guild-1",
		}}
	guildCreate := newGuildCreate(guild)
	session.State.GuildAdd(guildCreate.Guild)
	bot.guildCreate(session, guildCreate)
}

func TestGuildCreateTriggerArchive(t *testing.T) {
	session := newSessionStub()
	bot := newBot(session)
	guildID := "guild-1"
	guild := &discordgo.Guild{
		ID: guildID,
	}
	guild.Channels = []*discordgo.Channel{
		{
			ID:      "thread-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_ARCHIVE_CATEGORY_NAME + " 72",
			GuildID: guildID,
		},
		{
			ID:      "thread-archive-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_CATEGORY_NAME,
			GuildID: guildID,
		},
		{
			ID:       "channel-3",
			Type:     discordgo.ChannelTypeGuildCategory,
			Name:     "channel-3",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-4",
			Type:     discordgo.ChannelTypeGuildCategory,
			Name:     "channel-4",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-5",
			Type:     discordgo.ChannelTypeGuildCategory,
			Name:     "channel-5",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-6",
			Type:     discordgo.ChannelTypeGuildCategory,
			Name:     "channel-6",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-7",
			Type:     discordgo.ChannelTypeGuildCategory,
			Name:     "channel-7",
			GuildID:  guildID,
			ParentID: "thread-category",
		}}
	guildCreate := newGuildCreate(guild)
	session.State.GuildAdd(guildCreate.Guild)
	bot.guildCreate(session, guildCreate)
}
