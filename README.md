# gale-x-bot
Discord bot for GALE-x

## General Info

A bot in Discord is a particular applciaiton that starts a session that receives streaming websocket events as things happen. Those events contain information on just the things that changed, plus a bit more information.

discordgo is a go package that manages those sessions and also maintains a local copy of the implied Discord server state in discordgo.Session.State.

A bot will get messages for all discord servers (called Guilds in the API) that is connected to, and it's up to the bot to keep all of them straight. This bot typically only operates on one server at a time (either a dev server or a prod server).

A bot registers for the types of events that it wants to be notified of and receives callbacks when those kinds of events happen. By the time the callback runs, discordgo.Session.State will have already been updated to reflect the current state.

When a bot has a live session to a Discord server, it will show up in the Members list as being active, just like a real person would.

A bot connects to a server with a particular secret token specific to that bot ID. Only one instance of a given bot may connect to a server at a time.

You can read about the discordgo API at https://pkg.go.dev/github.com/bwmarrin/discordgo. You can read about the discord API at https://discord.com/developers/docs/resources/channel

## Setting up your own dev bot

When hacking on the bot, it's good to have your own dev version of the bot and a dev discord server that's just yours. That way your bot and state won't interact with others.

First, create a discord server.

Then, create your own application by going to https://discord.com/developers/applications and creating a new Application. Name it something like "GALEx dev bot".

Then, go to the Bot tab and hit add bot. You can give it whatever name you want. UNCHECK the 'Public bot' field and save.

You now must give your dev bot permission to join your dev Discord server.

Copy your application ID (in the URL of the page you're looking at, something like https://discord.com/developers/applications/837831972830576679/information and use that.)

Then, visit this URL (replacing the YOUR_APP_ID_HERE) with that app ID:

https://discord.com/api/oauth2/authorize?client_id=YOUR_APP_ID_HERE&scope=bot&permissions=268438544

Note that as this bot is developed, you might need to redo this connection if more permissions are ever required.

Now you need to get the secret token that allows you to authenticate as your bot.

Go to the Bot tab if you aren't already there and under the Token heading, click copy.

Open up a terminal window, and run `export BOT_TOKEN=<PASTED-TOKEN>`. (You can also pass the token on each invocation of the command by using the `-t` parameter).

Run the app by doing `go build && ./gale-x-bot` . It will connect to your dev server and print out debug information.

## Updating the production bot

The production bot is running in a `tmux` session on a Google Cloud VM running Ubuntu.

Connect via SSH to the server. Run `go install -u github.com/jkomoros/gale-x-bot`

Open up the tmux session via `tmux attach`. Kill the running one and then run the new one from the command history. Hit `Ctrl-b d` to disconnect from the tmux session and leave it running.

## Permissions

(The permissions number might need to be updated if it does more things. You can go to https://discord.com/developers/applications/837831972830576679/bot and use the tool at the bottom to calculate the permissions integer).

Current permissions it requires, in the permissions URL above:
 - Manage Channels
 - View Channels
 - Send Messages
 - Manage Roles