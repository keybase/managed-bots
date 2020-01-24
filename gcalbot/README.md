# Google Calendar Bot

I can connect to your Google calendar and notify you of invites, upcoming events and more! 📅

## Running

In order to run the Google Calendar bot, there needs to be a running MySQL database in
order to store account and webhook data.

1. On that SQL instance, create a database for the bot, and run `db.sql` to set
   up the tables.
2. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
3. Create an OAuth Client ID for a 'Web Application' via the [Google API
   Console](https://console.developers.google.com/apis/credentials). Download
   the credentials locally as `credentials.json`.
4. The bot sets itself up to serve HTTP requests on `/gcalbot`. The HTTP server
   runs on port 8080. You can configure nginx or any other reverse proxy
   software to route to this port and path.
5. To start the Google Calendar bot, run a command like this:
   ```
   # NOTE --kbfs-root specifies the path to the crendentials.json file.
   $GOPATH/bin/gcalbot --dsn 'root@/gcalbot' --kbfs-root ~/Downloads
   ```
6. Run `gcalbot --help` for more options.

### Helpful Tips

- If you accidentally run the bot under your own username and wish to clear the
  `!` commands, run the following:
  ```
  keybase chat api -m '{"method": "clearcommands"}'
  ```
- The following links are helpful for using the Google Calendar API:
    - https://developers.google.com/calendar/quickstart/go
    - https://developers.google.com/calendar/v3/reference

### Docker

There are a few complications running a Keybase chat bot, and it is likely
easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client
for our preferred client image to get started.
