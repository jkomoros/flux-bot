# gale-x-bot
Discord bot for GALE-x

## Info

You can get the token by visiting https://discord.com/developers/applications/837831972830576679/bot

You can then either set the token as an environment variable (`export BOT_TOKEN=<TOKEN>`) or pass it via the `-t` parameter.

Run the app by doing `go build && ./gale-x-bot`

## Adding a bot to a server

Have a user logged into a server visit this link:

https://discord.com/api/oauth2/authorize?client_id=837831972830576679&scope=bot&permissions=268438544

(The permissions number might need to be updated if it does more things. You can go to https://discord.com/developers/applications/837831972830576679/bot and use the tool at the bottom to calculate the permissions integer).

Current permissions it requires, in the permissions URL above:
 - Manage Channels
 - View Channels
 - Send Messages
 - Manage Roles