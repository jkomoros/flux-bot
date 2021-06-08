# flux-bot
Discord bot for FLUX

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

To create a bot:

1. Create a discord server.

2. Create your own application by going to https://discord.com/developers/applications and creating a new Application. Name it something like "FLUX dev bot".

3. Go to the Bot tab and hit add bot. You can give it whatever name you want. UNCHECK the 'Public bot' field and save.

You now must give your dev bot permission to join your dev Discord server.

1. Copy your application ID. You can find it in the URL of the page you're looking at, something like `https://discord.com/developers/applications/837831972830576679/information` and use that.

2. visit this URL (replacing the YOUR_APP_ID_HERE) with that app ID:
https://discord.com/api/oauth2/authorize?client_id=YOUR_APPLICATION_ID_HERE&permissions=2415922192&scope=applications.commands%20bot

Note that as this bot is developed, you might need to redo this connection if more permissions are ever required.

Now you need to get the secret token that allows you to authenticate as your bot.

1. Go to the Bot tab if you aren't already there and under the Token heading, click `Copy`.

1. Open up a terminal window, and run `export BOT_TOKEN=<PASTED-TOKEN>`. (You can also pass the token on each invocation of the command by using the `-t` parameter).

3. Run the app by doing `go build && ./flux-bot` . It will connect to your dev server and print out debug information.

## Hacking on application commands

Slash commands that are installed globally take an hour to roll out. When developing that can be a pain.

You can provide `debug-guild-id` option (or `DEBUG_GUILD_ID` env var) to register the command only to a specific debug guild during development.

## Updating the production bot

The production bot is running in a `tmux` session on a Google Cloud VM running Ubuntu.

1. Connect via SSH to the server. Run `go install github.com/jkomoros/flux-bot@latest`

2. Open up the tmux session via `tmux attach`. 
3. Kill the running one and then run the new one from the command history. 
4. Hit `Ctrl-b d` to disconnect from the tmux session and leave it running.

## Permissions

(The permissions number might need to be updated if it does more things. You can go to https://discord.com/developers/applications/837831972830576679/bot and use the tool at the bottom to calculate the permissions integer).

Current permissions it requires, in the permissions URL above:
 - Manage Channels
 - View Channels
 - Send Messages
 - Manage Roles