# GitHub Bot

A Keybase chat bot that notifies a channel when an event happens on a GitHub repository (issues, pull requests, commits, etc.).

## Prerequisites

In order to run the GitHub bot, you will need

- a running MySQL database in order to store GitHub OAuth tokens, user preferences, and channel subscriptions
- the client ID and client secret from a [GitHub OAuth application](https://developer.github.com/apps/building-oauth-apps/creating-an-oauth-app/)
- an arbitrary secret, used to authenticate webhooks from GitHub (this can be any string)

## Running

1. On your SQL instance, create a database for the bot, and run `db.sql` to set up the tables.
2. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
3. The GitHub bot sets itself up to serve HTTP requests on `/githubbot` plus a prefix indicating what the URLs will look like. The HTTP server runs on port 8080. You can configure nginx or any other reverse proxy software to route to this port and path. Make sure the callback url for your GitHub app is set to `http://<your web server>/githubbot/oauth`.
4. To start the GitHub bot, run a command like this:
   ```
   $GOPATH/bin/githubbot --http-prefix 'http://<your web server>:8080' --dsn 'root@/githubbot' --client-id '<OAuth client ID>' --client-secret '<OAuth client secret>' --secret '<your secret string>'
   ```
5. Run `githubbot --help` for more options.

### Helpful Tips

- If you accidentally run the bot under your own username and wish to clear the `!` commands, run the following:
  ```
  keybase chat api -m '{"method": "clearcommands"}'
  ```
- You can optionally save your GitHub OAuth ID and secret inside your bot account's private KBFS folder. To do this, create a `credentials.json` file in `/keybase/private/<YourGitHubBot>` (or the equivalent KBFS path on your system) that matches the following format:
  ```json
  {
    "client_id": "your GitHub OAuth client ID here",
    "client_secret": "your GitHub OAuth client secret here"
  }
  ```
  If you have KBFS running, you can now run the bot without providing the `--client-id` and `--client-secret` command line options.
- You can also store your bot secret in KBFS by saving it in a file named `bot.secret` in your bot account's private KBFS folder and omitting the `--secret` command line argument.

### Docker

There are a few complications running a Keybase chat bot, and it is likely easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client for our preferred client image to get started.
