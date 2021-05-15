package discord

import (
	"github.com/bwmarrin/discordgo"
)

type SessionTestDriver struct {
	ready              Ready
	guildCreate        GuildCreate
	messageCreate      MessageCreate
	channelCreate      ChannelCreate
	channelUpdate      ChannelUpdate
	HasUnknownHandlers bool
}

func NewSessionTestDriver() *SessionTestDriver {
	return &SessionTestDriver{}
}

func (s *SessionTestDriver) InvokeReady() bool {
	if s.ready == nil {
		return false
	}
	s.ready(s)
	return true
}

// discord.Session members

func (s *SessionTestDriver) AddHandler(handler interface{}) func() {
	switch v := handler.(type) {
	case func(Session):
		s.ready = Ready(v)
	case func(Session, *discordgo.GuildCreate):
		s.guildCreate = GuildCreate(v)
	case func(Session, *discordgo.MessageCreate):
		s.messageCreate = MessageCreate(v)
	case func(Session, *discordgo.ChannelCreate):
		s.channelCreate = ChannelCreate(v)
	case func(Session, *discordgo.ChannelUpdate):
		s.channelUpdate = ChannelUpdate(v)
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
