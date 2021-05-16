package main

import "github.com/bwmarrin/discordgo"

type Controller interface {
	GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error)
	ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error)
}

type DiscordController struct {
	session *discordgo.Session
}

func (dc *DiscordController) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (st *discordgo.Channel, err error) {
	return dc.session.GuildChannelCreateComplex(guildID, data)
}

func (dc *DiscordController) ChannelEditComplex(channelID string, data *discordgo.ChannelEdit) (st *discordgo.Channel, err error) {
	return dc.session.ChannelEditComplex(channelID, data)
}
