# Meet Bot

A Keybase chat bot that gives you a link to join a Google Meet video call.

## Running

In order to run the Meet bot, there needs to be a running MySQL database in order to store OAuth data.

1. On that SQL instance, create a database for the bot, and run `db.sql` to set
   up the tables.
2. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
3. Create an OAuth Client ID for a 'Web Application' via the [Google API
   Console](https://console.developers.google.com/apis/credentials). Download
   the credentials locally as `credentials.json`.
4. The bot sets itself up to serve HTTP requests on `/meetbot`. The HTTP server
   runs on port 8080. You can configure nginx or any other reverse proxy
   software to route to this port and path.
5. To start the Meet bot, run a command like this:
   ```
   # NOTE --kbfs-root specifies the path to the credentials.json file.
   $GOPATH/bin/meetbot --dsn 'root@/meetbot' --kbfs-root ~/Downloads
   ```
6. Run `meetbot --help` for more options.

### Helpful Tips

- If you accidentally run the bot under your own username and wish to clear the
  `!` commands, run the following:
  ```
  keybase chat clear-commands
  ```
- Restricted bots are restricted from knowing channel names. If you would like
  a bot to announce or report errors to a specific channel you can use a
  `ConversationID` which can be found by running:
  ```
  keybase chat conv-info teamname --channel channel
  ```
- By default, bots are unable to read their own messages. For development, it may be useful to disable this safeguard.
  You can do this using `--read-self` flag when running the bot.
- The following links are helpful for using the Google Calendar API:
    - https://developers.google.com/calendar/quickstart/go
    - https://developers.google.com/calendar/create-events

### Docker

There are a few complications running a Keybase chat bot, and it is likely
easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client
for our preferred client image to get started.

