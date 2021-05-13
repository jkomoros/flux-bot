package discord

import (
	"github.com/bwmarrin/discordgo"
)

// Interfaces that describe the currently used Discord API surface

type Session interface {
	AddHandler(handler interface{}) func()
	GetState() *discordgo.State
	GuildChannelsReorder(guildID string, channels []*discordgo.Channel) error
	GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error)
	ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error)
}
