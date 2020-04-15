# Zoom Bot

A Keybase chat bot that gives you a link to join a Zoom instant meeting.

## Running

In order to run the Zoom bot, there needs to be a running MySQL database in order to store OAuth data.

1. On that SQL instance, create a database for the bot, and run `db.sql` to set
   up the tables.
2. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
3. Create an OAuth App on the [Zoom Marketplace](https://marketplace.zoom.us/develop/create). Fill in all of
the necessary details (name, description, etc.). Additionally, set the redirect URL *and* whitelist as
`https://mydomain.com/zoombot/oauth`, add `user:read` and `meeting:write` to the Scopes and set the deauthorization
notification endpoint URL as `https://mydomain.com/zoombot/deauthorize`.
4. The bot sets itself up to serve HTTP requests on `/zoombot`. The HTTP server
   runs on port 8080. You can configure nginx or any other reverse proxy
   software to route to this port and path.
5. To start the Zoom bot, run a command like this:
   ```
   $GOPATH/bin/zoombot --dsn 'root@/zoombot' --http-prefix https://mydomain.com --client-id '<OAuth client ID>' --client-secret '<OAuth client secret>' --verification-token '<Zoom Verification Token>'
   ```
6. Run `zoombot --help` for more options.

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
- You can optionally save your Zoom credentials inside your bot account's private KBFS folder.
To do this, create a `credentials.json` and use the `--kbfs-root` flag to specify the folder that it's in
(example: `--kbfs-root /keybase/private/<YourZoomBot>`). The `credentials.json` file should follow this format:
  ```json
  {
    "client_id": "your Zoom OAuth client ID here",
    "client_secret": "your Zoom OAuth client secret here",
    "verification_token": "your Zoom verification token here"
  }
  ```
  If you have KBFS running, you can now run the bot without providing the `--client-id`, `--client-secret` and `--verification-token` command line options.
- The following links are helpful for using the Zoom API:
    - https://marketplace.zoom.us/docs/guides/getting-started/app-types/create-oauth-app
    - https://marketplace.zoom.us/docs/api-reference/zoom-api/meetings/meetingcreate
    - https://marketplace.zoom.us/docs/guides/authorization/deauthorization

### Docker

There are a few complications running a Keybase chat bot, and it is likely
easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client
for our preferred client image to get started.

