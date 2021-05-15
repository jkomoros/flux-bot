package main

import (
	"testing"

	"github.com/jkomoros/gale-x-bot/discord"
	"github.com/stretchr/testify/assert"
)

func TestBotInstantiation(t *testing.T) {
	driver := discord.NewTestDriver()
	newBot(driver)
	// if the assertion below fails, you probably added new API callbacks.
	// Add them to wrappers.go and test_harness.go to make this test pass.
	assert.False(t, driver.HasUnknownHandlers, "should not register unknown callbacks")
	// if this assertion fails, bot no longer registers the "ready" callback.
	assert.True(t, driver.InvokeReady(), "ready handler should be set and invokable")
}
