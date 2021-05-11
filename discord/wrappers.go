package discord

import (
	"github.com/bwmarrin/discordgo"
)

type SessionWrapper struct {
	session *discordgo.Session
}

func NewSessionWrapper(session *discordgo.Session) *SessionWrapper {
	return &SessionWrapper{session}
}

func (s *SessionWrapper) AddHandler(handler interface{}) func() {
	return s.session.AddHandler(handler)
}

func (s *SessionWrapper) GetState() *discordgo.State {
	return s.session.State
}

func (s *SessionWrapper) GuildChannelsReorder(guildID string, channels []*discordgo.Channel) error {
	return s.session.GuildChannelsReorder(guildID, channels)
}

func (s *SessionWrapper) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error) {
	return s.session.GuildChannelCreateComplex(guildID, data)
}

func (s *SessionWrapper) ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error) {
	return s.session.ChannelEditComplex(channelID, data)
}
