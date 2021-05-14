package main

import (
	"testing"

	"github.com/jkomoros/gale-x-bot/discord"
	"github.com/stretchr/testify/assert"
)

func TestBotInstantiation(t *testing.T) {
	session := discord.NewSessionTestDriver()
	newBot(session)
	// if the assertion below fails, you probably added new API callbacks.
	// Add them to to test driver to make it pass.
	assert.False(t, session.HasUnknownHandlers, "should not register unknown callbacks")
	// if this assertion fails, bot no longer registers the "ready" callback.
	assert.True(t, session.InvokeReady(), "ready handler should be set and invokable")
}
