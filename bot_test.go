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

func TestGuildCreate(t *testing.T) {
	session := newSessionStub()
	bot := newBot(session)
	guild := &discordgo.Guild{
		ID: "guild-1",
	}
	// GuildID is necessary in archiveThreadsIfNecessary
	// TODO: maybe find a way to not need it
	guild.Channels = append(guild.Channels, &discordgo.Channel{
		ID:      "channel-1",
		Type:    discordgo.ChannelTypeGuildCategory,
		Name:    THREAD_ARCHIVE_CATEGORY_NAME + " 72",
		GuildID: "guild-1",
	})

	guild.Channels = append(guild.Channels, &discordgo.Channel{
		ID:      "channel-2",
		Type:    discordgo.ChannelTypeGuildCategory,
		Name:    THREAD_CATEGORY_NAME,
		GuildID: "guild-1",
	})
	guildCreate := newGuildCreate(guild)
	session.State.GuildAdd(guildCreate.Guild)
	bot.guildCreate(session, guildCreate)
}
