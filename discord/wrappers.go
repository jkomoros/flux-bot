package discord

import (
	"github.com/bwmarrin/discordgo"
)

type sessionWrapper struct {
	session *discordgo.Session
}

func NewSessionWrapper(session *discordgo.Session) *sessionWrapper {
	return &sessionWrapper{session}
}

func (s *sessionWrapper) AddHandler(handler interface{}) func() {
	return s.session.AddHandler(handler)
}

func (s *sessionWrapper) GetState() *discordgo.State {
	return s.session.State
}

func (s *sessionWrapper) ChannelMessage(channelID, messageID string) (st *discordgo.Message, err error) {
	return s.session.ChannelMessage(channelID, messageID)
}

func (s *sessionWrapper) ChannelMessages(channelID string, limit int, beforeID, afterID, aroundID string) (st []*discordgo.Message, err error) {
	return s.session.ChannelMessages(channelID, limit, beforeID, afterID, aroundID)
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

type sessionStubWrapper struct {
}

func NewSessionStubWrapper() *sessionStubWrapper {
	return nil
}

func (s *sessionStubWrapper) AddHandler(handler interface{}) func() {
	return nil
}

func (s *sessionStubWrapper) GetState() *discordgo.State {
	return nil
}

func (s *sessionStubWrapper) ChannelMessage(channelID, messageID string) (st *discordgo.Message, err error) {
	return nil, nil
}

func (s *sessionStubWrapper) ChannelMessages(channelID string, limit int, beforeID, afterID, aroundID string) (st []*discordgo.Message, err error) {
	return nil, nil
}

func (s *sessionStubWrapper) GuildChannelsReorder(guildID string, channels []*discordgo.Channel) error {
	return nil
}

func (s *sessionStubWrapper) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error) {
	return nil, nil
}

func (s *sessionStubWrapper) ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error) {
	return nil, nil
}
