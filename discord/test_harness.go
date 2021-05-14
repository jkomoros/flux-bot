package discord

import (
	"github.com/bwmarrin/discordgo"
)

type readyEventHandler func(*discordgo.Session, *discordgo.Ready)
type guildCreateEventHandler func(*discordgo.Session, *discordgo.GuildCreate)
type messageCreateHandler func(*discordgo.Session, *discordgo.MessageCreate)
type channelCreateHandler func(*discordgo.Session, *discordgo.ChannelCreate)
type channelUpdateHandler func(*discordgo.Session, *discordgo.ChannelUpdate)

type SessionTestDriver struct {
	ready              readyEventHandler
	guildCreate        guildCreateEventHandler
	messageCreate      messageCreateHandler
	channelCreate      channelCreateHandler
	channelUpdate      channelUpdateHandler
	HasUnknownHandlers bool
}

func NewSessionTestDriver() *SessionTestDriver {
	return &SessionTestDriver{}
}

func (s *SessionTestDriver) InvokeReady() bool {
	if s.ready == nil {
		return false
	}
	s.ready(nil, nil)
	return true
}

// discord.Session members

func (s *SessionTestDriver) AddHandler(handler interface{}) func() {
	switch v := handler.(type) {
	case func(*discordgo.Session, *discordgo.Ready):
		s.ready = readyEventHandler(v)
	case func(*discordgo.Session, *discordgo.GuildCreate):
		s.guildCreate = guildCreateEventHandler(v)
	case func(*discordgo.Session, *discordgo.MessageCreate):
		s.messageCreate = messageCreateHandler(v)
	case func(*discordgo.Session, *discordgo.ChannelCreate):
		s.channelCreate = channelCreateHandler(v)
	case func(*discordgo.Session, *discordgo.ChannelUpdate):
		s.channelUpdate = channelUpdateHandler(v)
	default:
		s.HasUnknownHandlers = true
	}
	return nil
}

func (s *SessionTestDriver) GetState() *discordgo.State {
	return nil
}

func (s *SessionTestDriver) GuildChannelsReorder(guildID string, channels []*discordgo.Channel) error {
	return nil
}

func (s *SessionTestDriver) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error) {
	return nil, nil
}

func (s *SessionTestDriver) ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error) {
	return nil, nil
}
