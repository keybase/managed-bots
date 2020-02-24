# Canary Bot

A simple example Keybase chat bot to get started with. The bot showcases some
basic features including advertising bot commands and responding to user input.
The bot's primary purpose is to be the 'canary in the coal mine' for the
managed bot's infrastructure, it is a safe place to try out new features that
the bots use.

## Running

1. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
2. Canary bot sets itself up to serve HTTP requests on `/canarybot`. The HTTP server runs on port 8080. You can configure nginx or any other reverse proxy software to route to this port and path.
3. To start the Canary bot, run a command like this:
   ```
   $GOPATH/bin/canarybot
   ```
4. Run `canarybot --help` for more options.

### Helpful Tips

- If you accidentally run the bot under your own username and wish to clear the `!` commands, run the following:
  ```
  keybase chat clear-commands
  ```
- Restricted bots are restricted from knowing channel names. If you would like
  a bot to announce or report errors to a specific channel you can use a
  `ConversationID` which can be found by running:
  ```
  keybase chat conv-info [teamname] --channel channel_name
  ```

### Docker

There are a few complications running a Keybase chat bot, and it is likely easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client for our preferred client image to get started.
