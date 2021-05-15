package discord

import (
	"github.com/bwmarrin/discordgo"
)

// These types wrap around the discordgo API to enable testing and to track used API surface.

type sessionWrapper struct {
	session *discordgo.Session
}

func NewSession(session *discordgo.Session) *sessionWrapper {
	return &sessionWrapper{session}
}

// Event handler wrapper types
// Anytime you start listening to a new event, wrap it here
type Ready func(Session)
type GuildCreate func(Session, *discordgo.GuildCreate)
type MessageCreate func(Session, *discordgo.MessageCreate)
type ChannelCreate func(Session, *discordgo.ChannelCreate)
type ChannelUpdate func(Session, *discordgo.ChannelUpdate)

func (s *sessionWrapper) AddHandler(handler interface{}) func() {
	switch v := handler.(type) {
	case func(Session):
		return s.session.AddHandler(func(session *discordgo.Session, event *discordgo.Ready) {
			ready := Ready(v)
			ready(NewSession(session))
		})
	case func(Session, *discordgo.GuildCreate):
		return s.session.AddHandler(func(session *discordgo.Session, event *discordgo.GuildCreate) {
			guildCreate := GuildCreate(v)
			guildCreate(NewSession(session), event)
		})
	case func(Session, *discordgo.MessageCreate):
		return s.session.AddHandler(func(session *discordgo.Session, event *discordgo.MessageCreate) {
			messageCreate := MessageCreate(v)
			messageCreate(NewSession(session), event)
		})
	case func(Session, *discordgo.ChannelCreate):
		return s.session.AddHandler(func(session *discordgo.Session, event *discordgo.ChannelCreate) {
			channelCreate := ChannelCreate(v)
			channelCreate(NewSession(session), event)
		})
	case func(Session, *discordgo.ChannelUpdate):
		return s.session.AddHandler(func(session *discordgo.Session, event *discordgo.ChannelUpdate) {
			channelCreate := ChannelUpdate(v)
			channelCreate(NewSession(session), event)
		})
	}
	return nil
}

func (s *sessionWrapper) GetState() *discordgo.State {
	return s.session.State
}

func (s *sessionWrapper) GuildChannelsReorder(guildID string, channels []*discordgo.Channel) error {
	return s.session.GuildChannelsReorder(guildID, channels)
}

func (s *sessionWrapper) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error) {
	return s.session.GuildChannelCreateComplex(guildID, data)
}

func (s *sessionWrapper) ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error) {
	return s.session.ChannelEditComplex(channelID, data)
}
