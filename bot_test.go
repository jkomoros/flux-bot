package main

import (
	"testing"

	"github.com/jkomoros/gale-x-bot/discord"
)

func TestBot(t *testing.T) {
	newBot(discord.NewSessionStubWrapper())
}
