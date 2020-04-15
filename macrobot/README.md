# Macro Bot

A Keybase chat bot that can create and run simple macros.

## Running

In order to run the Macro bot, there needs to be a running MySQL database in order to store the registered macros.

1. On that SQL instance, create a database for the bot, and run `db.sql` to set up the tables.
2. Build the bot using Go 1.13+, like such (in this directory):
   ```
   go install .
   ```
3. To start the Macro bot, run a command like this:
   ```
   $GOPATH/bin/macrobot --dsn 'root@/macrobot?charset=utf8mb4'
   ```
4. Run `macrobot --help` for more options.

### Helpful Tips

- If you accidentally run the bot under your own username and wish to clear the `!` commands, run the following:
  ```
  keybase chat clear-commands
  ```
- Restricted bots are restricted from knowing channel names. If you would like
  a bot to announce or report errors to a specific channel you can use a
  `ConversationID` which can be found by running:
  ```
  keybase chat conv-info teamname --channel channel
  ```

### Docker

There are a few complications running a Keybase chat bot, and it is likely easiest to deploy using Docker. See https://hub.docker.com/r/keybaseio/client for our preferred client image to get started.
