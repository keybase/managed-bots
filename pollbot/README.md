# Poll Bot

A Keybase chat bot that allows members of conversations to easily create both public and anonymous polls.

## Running

In order to run the Poll bot, there needs to be a running MySQL database in order to store the currently active polls and enforce single-vote anonymous polls.

1. On that SQL instance, create a database for the bot, and run `db.sql` to set up the tables.
2. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
3. Poll bot sets itself up to serve HTTP requests on `/pollbot` plus a prefix indicating what the anonymous voting URLs will look like. The HTTP server runs on port 8080. You can configure nginx or any other reverse proxy software to route to this port and path.
4. In order for users to login to vote anonymously in a web browser, the Poll bot needs a server side secret. You must pass this in with `--login-secret`.
5. To start the Poll bot, run a command like this:
   ```
   $GOPATH/bin/pollbot --http-prefix 'http://localhost:8080' --dsn 'root@/webhookbot' --login-secret 'moony wormtail padfoot prongs'
   ```
6. Run `pollbot --help` for more options.

### Helpful Tips

- If you accidentally run the bot under your own username and wish to clear the `!` commands, run the following:
  ```
  keybase chat api -m '{"method": "clearcommands"}'
  ```

### Docker

There are a few complications running a Keybase chat bot, and it is likely easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client for our preferred client image to get started.
