package main

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

const (
	TEST_TOKEN    string = "fake-value"
	TEST_GUILD_ID string = "guild-1"
)

func newSessionStub() *discordgo.Session {
	session, _ := discordgo.New(TEST_TOKEN)
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
	session                            *discordgo.Session
	channelEditComplexCallCount        int
	guildChannelCreateComplexCallCount int
	guildChannelReorderCallCount       int
}

func (tc *TestController) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error) {
	tc.guildChannelCreateComplexCallCount++
	channel := &discordgo.Channel{
		ID:      "thread-archive-category",
		Type:    discordgo.ChannelTypeGuildCategory,
		Name:    THREAD_ARCHIVE_CATEGORY_NAME + " 13",
		GuildID: TEST_GUILD_ID,
	}
	tc.session.State.ChannelAdd(channel)
	return channel, nil
}
func (tc *TestController) ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error) {
	tc.channelEditComplexCallCount++
	return nil, nil
}

func (tc *TestController) GuildChannelsReorder(guildID string, channels []*discordgo.Channel) (err error) {
	tc.guildChannelReorderCallCount++
	return nil
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
		ID: TEST_GUILD_ID,
	}
	// GuildID is necessary in archiveThreadsIfNecessary
	// TODO: maybe find a way to not need it
	guild.Channels = []*discordgo.Channel{
		{
			ID:      "thread-archive-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_ARCHIVE_CATEGORY_NAME + " 72",
			GuildID: TEST_GUILD_ID,
		},
		{
			ID:      "thread-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_CATEGORY_NAME,
			GuildID: TEST_GUILD_ID,
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
	guild := &discordgo.Guild{
		ID: TEST_GUILD_ID,
	}
	guild.Channels = []*discordgo.Channel{
		{
			ID:      "thread-archive-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_ARCHIVE_CATEGORY_NAME + " 72",
			GuildID: TEST_GUILD_ID,
		},
		{
			ID:      "thread-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_CATEGORY_NAME,
			GuildID: TEST_GUILD_ID,
		},
		{
			ID:       "channel-1",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-1",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-2",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-2",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-3",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-3",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-4",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-4",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-5",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-5",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-6",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-6",
			GuildID:  TEST_GUILD_ID,
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

func TestGuildCreateTriggerCreatingArchivedCategory(t *testing.T) {
	session := newSessionStub()
	maxActiveThreads = 1
	controller := &TestController{}
	controller.session = session
	bot := newBot(session, controller)
	guild := &discordgo.Guild{
		ID: TEST_GUILD_ID,
	}
	guild.Roles = []*discordgo.Role{
		{
			ID:   "everyone-role",
			Name: EVERYONE_ROLE_NAME,
		},
	}
	guild.Channels = []*discordgo.Channel{
		{
			ID:      "thread-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_CATEGORY_NAME,
			GuildID: TEST_GUILD_ID,
		},
		{
			ID:       "channel-1",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-1",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-2",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-2",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
	}
	session.State.GuildAdd(guild)
	bot.guildCreate(session, newGuildCreate((guild)))
	if controller.channelEditComplexCallCount != 1 {
		t.Errorf("ChannelEditComplex should have been called once.")
	}
	if controller.guildChannelCreateComplexCallCount != 1 {
		t.Errorf("GuildChannelCreateComplex should have been called once.")
	}
}

func TestMessageCreate(t *testing.T) {
	session := newSessionStub()
	maxActiveThreads = 1
	controller := &TestController{}
	controller.session = session
	bot := newBot(session, controller)
	guild := &discordgo.Guild{
		ID: TEST_GUILD_ID,
	}
	guild.Channels = []*discordgo.Channel{
		{
			ID:      "thread-category",
			Type:    discordgo.ChannelTypeGuildCategory,
			Name:    THREAD_CATEGORY_NAME,
			GuildID: TEST_GUILD_ID,
		},
		{
			ID:       "channel-1",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-1",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
		{
			ID:       "channel-2",
			Type:     discordgo.ChannelTypeGuildText,
			Name:     "channel-2",
			GuildID:  TEST_GUILD_ID,
			ParentID: "thread-category",
		},
	}
	session.State.GuildAdd(guild)
	bot.messageCreate(session, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-1",
			ChannelID: "channel-2",
			GuildID:   TEST_GUILD_ID,
			// Content:          "",
			// Timestamp:        "",
			// EditedTimestamp:  "",
			// MentionRoles:     []string{},
			// TTS:              false,
			// MentionEveryone:  false,
			// Author:           &discordgo.User{},
			// Attachments:      []*discordgo.MessageAttachment{},
			// Embeds:           []*discordgo.MessageEmbed{},
			// Mentions:         []*discordgo.User{},
			// Reactions:        []*discordgo.MessageReactions{},
			// Pinned:           false,
			// Type:             0,
			// WebhookID:        "",
			// Member:           &discordgo.Member{},
			// MentionChannels:  []*discordgo.Channel{},
			// Activity:         &discordgo.MessageActivity{},
			// Application:      &discordgo.MessageApplication{},
			// MessageReference: &discordgo.MessageReference{},
			// Flags:            0,
		},
	})
	if controller.guildChannelReorderCallCount != 1 {
		t.Errorf("ChannelEditComplex should have been called once.")
	}
}
