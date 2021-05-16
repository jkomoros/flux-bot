package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

const APP_NAME = "gale-x-bot"
const TOKEN_ENV_NAME = "BOT_TOKEN"
const MAX_ACTIVE_THREADS_ENV_NAME = "BOT_MAX_THREADS"
const DEBUG_GUILD_ID_ENV_NAME = "DEBUG_GUILD_ID"

//The name of the category this will look for that contains things this should treat as threads
const THREAD_CATEGORY_NAME = "Threads"
const THREAD_ARCHIVE_CATEGORY_NAME = THREAD_CATEGORY_NAME + " Archive"

//Number of threads to allow in an active category before bumping old ones.
//Should be at least one smaller than MAX_CATEGORY_CHANNELS.
const DEFAULT_MAX_ACTIVE_THREADS = 5

//The max number of channels that Discord allows to be in a category channel. This is configured by discord.
const MAX_CATEGORY_CHANNELS = 50
const EVERYONE_ROLE_NAME = "@everyone"

var token string
var maxActiveThreads int
var debugGuildIDForCommand string

const FORK_COMMAND_NAME = "fork"

var (
	//When creating a command also update bot.interactionCreate to dispatch to the handler for the interaction
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        FORK_COMMAND_NAME,
			Description: "Create a new thread with an OP copied from the message this slash command is in reply to",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "title",
					Description: "title of the thread to create",
					Required:    true,
				},
			},
		},
	}
)

func main() {
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.IntVar(&maxActiveThreads, "n", -1, "Max number of threads per group")
	flag.StringVar(&debugGuildIDForCommand, "debug-guild-id", "", "The guild ID to register commands with, useful during testing since global commands take an hour to roll out")
	flag.Parse()

	if token == "" {
		token = os.Getenv(TOKEN_ENV_NAME)
	}

	if token == "" {
		fmt.Println("No token provided. Please run: " + APP_NAME + " -t <bot token> or set env var " + TOKEN_ENV_NAME)
		return
	}

	if maxActiveThreads == -1 {
		maxActiveThreadsStr := os.Getenv(MAX_ACTIVE_THREADS_ENV_NAME)
		if maxActiveThreadsStr != "" {
			var err error
			maxActiveThreads, err = strconv.Atoi(maxActiveThreadsStr)
			if err != nil {
				fmt.Printf("Invalid int provided for max_active_threads: %v", err)
				return
			}
		}
	}

	if maxActiveThreads == -1 {
		fmt.Println("No max_active_threads provided. Defaulting to " + strconv.Itoa(DEFAULT_MAX_ACTIVE_THREADS) + ". You can provide it with -n or set env var " + MAX_ACTIVE_THREADS_ENV_NAME)
		maxActiveThreads = DEFAULT_MAX_ACTIVE_THREADS
	}

	if debugGuildIDForCommand == "" {
		debugGuildIDForCommand = os.Getenv(DEBUG_GUILD_ID_ENV_NAME)
		if debugGuildIDForCommand != "" {
			fmt.Println("Using " + DEBUG_GUILD_ID_ENV_NAME + " env var: " + debugGuildIDForCommand)
		}
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	// Register ready as a callback for the ready events.
	bot := newBot(dg, &DiscordController{dg})

	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
		return
	}
	defer dg.Close()

	if err = bot.registerSlashCommands(); err != nil {
		fmt.Printf("Couldn't register slash commands: %v", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println(APP_NAME + " is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
