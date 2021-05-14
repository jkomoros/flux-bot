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

func (s *sessionWrapper) AddHandler(handler interface{}) func() {
	return s.session.AddHandler(handler)
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
