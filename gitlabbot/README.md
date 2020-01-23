# GitLab Bot

A Keybase chat bot that notifies a channel when an event happens on a GitLab project (issues, pull requests, commits, etc.).

## GitLab API

The one scope needed for GitLab bot is: `api`. For more information on GitLab scopes visit the GitLab
[docs](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html#limiting-scopes-of-a-personal-access-token).

In GitLab's OAuth Application dashboard specify the Callback URL: `https://<YOUR_DOMAIN>/gitlabbot/oauth`
which is needed for the redirect in the OAuth flow.

> Note `<YOUR_DOMAIN>` should reflect where GitLab bot is running (same as `http-prefix` flag under **Running**)

## Prerequisites

In order to run the GitLab bot, you will need

- A running MySQL database in order to store GitLab OAuth tokens, user preferences, and channel subscriptions
- The client ID and client secret from a [GitLab OAuth application](https://docs.gitlab.com/ee/integration/oauth_provider.html)
- An arbitrary secret, used to authenticate webhooks from GitLab (this can be any string)

## Running

1. On your SQL instance, create a database for the bot, and run `db.sql` to set up the tables.
2. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
3. The GitLab bot sets itself up to serve HTTP requests on `/gitlabbot` plus a prefix indicating what the URLs will look like. The HTTP server runs on port 8080. You can configure nginx or any other reverse proxy software to route to this port and path. Make sure the callback url for your GitLab app is set to `http://<your web server>/gitlabbot/oauth`.
4. To start the GitLab bot, run a command like this:
   ```
   $GOPATH/bin/gitlabbot --http-prefix 'http://<YOUR_DOMAIN>:8080' --dsn 'root@/gitlabbot' --client-id '<OAuth client ID>' --client-secret '<OAuth client secret>' --secret '<your secret string>'
   ```
5. Run `gitlabbot --help` for more options.

### Helpful Tips

- [ngrok](https://ngrok.com) provides temporary web urls that can serve from localhost, which means you can use ngrok to test locally. You will need to add your ngrok generated url to the Callback URL section of your GitLab OAuth app. As well as use that as the `http-prefix` flag when running the bot.
- If you accidentally run the bot under your own username and wish to clear the `!` commands, run the following:
  ```
  keybase chat api -m '{"method": "clearcommands"}'
  ```
- You can optionally save your GitLab OAuth ID , OAuth secret, and bot secret inside your bot account's private KBFS folder. To do this, create a `credentials.json` file in `/keybase/private/<YourGitLabBot>` (or the equivalent KBFS path on your system) that matches the following format:
  ```json
  {
    "client_id": "your GitLab OAuth client ID here",
    "client_secret": "your GitLab OAuth client secret here",
    "webhook_secret": "your secret here"
  }
  ```
  If you have KBFS running, you can now run the bot without providing the `--client-id`, `--client-secret`, and `--secret` command line options.

### Docker

There are a few complications running a Keybase chat bot, and it is likely easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client for our preferred client image to get started.
