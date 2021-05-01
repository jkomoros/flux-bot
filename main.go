package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func init() {
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()
}

const APP_NAME = "gale-x-bot"
const TOKEN_ENV_NAME = "BOT_TOKEN"

//The name of the category this will look for that contains things this should treat as threads
const THREAD_CATEGORY_NAME = "Threads"

var token string

type bot struct {
	session    *discordgo.Session
	guildInfos map[string]*guildInfo
}

type guildInfo struct {
	threadCategoryID string
}

func main() {

	if token == "" {
		token = os.Getenv(TOKEN_ENV_NAME)
	}

	if token == "" {
		fmt.Println("No token provided. Please run: " + APP_NAME + " -t <bot token> or set env var " + TOKEN_ENV_NAME)
		return
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	// Register ready as a callback for the ready events.
	newBot(dg)

	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println(APP_NAME + " is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

func newBot(s *discordgo.Session) *bot {
	result := &bot{
		session:    s,
		guildInfos: make(map[string]*guildInfo),
	}
	s.AddHandler(result.ready)
	s.AddHandler(result.guildCreate)
	s.AddHandler(result.messageCreate)
	return result
}

// This function will be called (due to AddHandler above) when the bot receives
// the "ready" event from Discord.
func (b *bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	//GuildInfo isn't populated yet.
	fmt.Println("Ready and waiting!")
}

//This will be called after the bot starts up for each guild it's added to
func (b *bot) guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	b.inductGuild(event.Guild)
}

func (b *bot) messageCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
	channel, err := s.Channel(event.ChannelID)
	if err != nil {
		fmt.Println("Couldn't find channel")
		return
	}
	gi := b.guildInfos[event.GuildID]
	if gi == nil {
		//Must be a message from a server without a Threads category
		return
	}
	if channel.ParentID != gi.threadCategoryID {
		//A message outside of Threads category
		return
	}
	if err := b.moveThreadToTopOfThreads(channel); err != nil {
		fmt.Printf("message received in a thread but couldn't move it: %v", err)
	}
}

func (b *bot) inductGuild(guild *discordgo.Guild) {

	var threadsCategory *discordgo.Channel

	for _, channel := range guild.Channels {
		if channel.Type != discordgo.ChannelTypeGuildCategory {
			continue
		}
		if channel.Name == THREAD_CATEGORY_NAME {
			threadsCategory = channel
			continue
		}
	}

	if threadsCategory == nil {
		fmt.Println(guild.Name + " (ID " + guild.ID + ") joined but didn't have a category named " + THREAD_CATEGORY_NAME)
		return
	}

	fmt.Println("Found " + THREAD_CATEGORY_NAME + " category in guild " + nameForGuild(guild))

	info := &guildInfo{
		threadCategoryID: threadsCategory.ID,
	}

	b.guildInfos[guild.ID] = info

}

func (b *bot) moveThreadToTopOfThreads(thread *discordgo.Channel) error {

	fmt.Println("Popping thread " + nameForThread(thread) + " to top because it received a new message")

	guild, err := b.session.State.Guild(thread.GuildID)
	if err != nil {
		return fmt.Errorf("couldn't fetch guild: %w", err)
	}
	gi := b.guildInfos[guild.ID]
	if gi == nil {
		return fmt.Errorf("guild didn't have threads")
	}
	var threads []*discordgo.Channel
	//the thread we want to move to the head, but refreshed
	var headThread *discordgo.Channel
	for _, channel := range guild.Channels {
		if channel.ParentID == gi.threadCategoryID {
			if channel.ID == thread.ID {
				headThread = channel
			} else {
				threads = append(threads, channel)
			}
		}
	}

	if headThread == nil {
		return fmt.Errorf("didn't find the target thread unexpectedly")
	}

	threads = append([]*discordgo.Channel{headThread}, threads...)

	for i, channel := range threads {
		channel.Position = i
	}

	if err := b.session.GuildChannelsReorder(guild.ID, threads); err != nil {
		return fmt.Errorf("couldn't reorder channels: %w", err)
	}

	return nil
}

func nameForGuild(guild *discordgo.Guild) string {
	return guild.Name + " (" + guild.ID + ")"
}

func nameForThread(thread *discordgo.Channel) string {
	return thread.Name + " (" + thread.ID + ")"
}
