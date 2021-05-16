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

type TestController struct {
	channelEditComplexCallCount        int
	guildChannelCreateComplexCallCount int
}

func (tc *TestController) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error) {
	tc.guildChannelCreateComplexCallCount++
	return nil, nil
}
func (tc *TestController) ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error) {
	tc.channelEditComplexCallCount++
	return nil, nil
}

func TestReady(t *testing.T) {
	session := newSessionStub()
	bot := newBot(session, &TestController{})
	bot.ready(session, newReadyEvent())
}

func TestGuildCreateSimple(t *testing.T) {
	session := newSessionStub()
	controller := &TestController{}
	bot := newBot(session, controller)
	guild := &discordgo.Guild{
		ID: "guild-1",
	}
	// GuildID is necessary in archiveThreadsIfNecessary
	// TODO: maybe find a way to not need it
	guild.Channels = []*discordgo.Channel{
		{
			ID:      "thread-archive-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_ARCHIVE_CATEGORY_NAME + " 72",
			GuildID: "guild-1",
		},
		{
			ID:      "thread-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_CATEGORY_NAME,
			GuildID: "guild-1",
		}}
	session.State.GuildAdd(guild)
	bot.guildCreate(session, newGuildCreate((guild)))
	if controller.channelEditComplexCallCount != 0 {
		t.Errorf("ChannelEditComplex should not have been called.")
	}
	if controller.guildChannelCreateComplexCallCount != 0 {
		t.Errorf("GuildChannelCreateComplex should not have been called.")
	}
}

func TestGuildCreateTriggerArchive(t *testing.T) {
	session := newSessionStub()
	maxActiveThreads = 5
	controller := &TestController{}
	bot := newBot(session, controller)
	guildID := "guild-1"
	guild := &discordgo.Guild{
		ID: guildID,
	}
	guild.Channels = []*discordgo.Channel{
		{
			ID:      "thread-archive-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_ARCHIVE_CATEGORY_NAME + " 72",
			GuildID: guildID,
		},
		{
			ID:      "thread-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_CATEGORY_NAME,
			GuildID: guildID,
		},
		{
			ID:       "channel-1",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-1",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-2",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-2",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-3",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-3",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-4",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-4",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-5",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-5",
			GuildID:  guildID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-6",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-6",
			GuildID:  guildID,
			ParentID: "thread-category",
		}}
	session.State.GuildAdd(guild)
	bot.guildCreate(session, newGuildCreate((guild)))
	if controller.channelEditComplexCallCount != 1 {
		t.Errorf("ChannelEditComplex should have been called once.")
	}
	if controller.guildChannelCreateComplexCallCount != 0 {
		t.Errorf("GuildChannelCreateComplex should not have been called.")
	}
}
