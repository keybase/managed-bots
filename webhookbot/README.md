# Webhook Bot

A Keybase chat bot that provides a simple webhook interface to connect outside programs into Keybase chat easily.

## Running

In order to run the Webhook bot, there needs to be a running MySQL database in order to store the set of hooks.

1. On that SQL instance, create a database for the bot, and run `db.sql` to set up the tables.
2. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
3. Webhook bot sets itself up to serve HTTP requests on `/webhookbot` plus a prefix indicating what the URLs will look like. The HTTP server runs on port 8080. You can configure nginx or any other reverse proxy software to route to this port and path.
4. To start the Webhook bot, run a command like this:
   ```
   $GOPATH/bin/webhookbot --http-prefix 'http://localhost:8080' --dsn 'root@/webhookbot'
   ```
5. Run `webhookbot --help` for more options.

### Helpful Tips

- If you accidentally run the bot under your own username and wish to clear the `!` commands, run the following:
  ```
  keybase chat clear-commands
  ```

### Docker

There are a few complications running a Keybase chat bot, and it is likely easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client for our preferred client image to get started.
