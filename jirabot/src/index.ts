import Bot from './bot'
import {init} from './context'
import * as BotConfig from './bot-config'
import http from 'http'
import url from 'url'
import logger from './logger'
import {onJiraCallback} from './jira-oauth'
import * as Constants from './constants'

const handleElbHealthCheck = () =>
  http
    .createServer((req, res) => {
      try {
        if (req.url === Constants.healthCheckPathname) {
          res.write('ok')
          res.end()
          return
        }

        const parsedUrl = url.parse(req.url, true)
        if (parsedUrl.pathname === Constants.jiraOauthCallbackPathname) {
          const {oauth_token, oauth_verifier} = parsedUrl.query
          if (
            typeof oauth_token !== 'string' ||
            typeof oauth_verifier !== 'string'
          ) {
            res.writeHead(400)
            res.end('unexpected callback data')
            return
          }
          onJiraCallback(oauth_token, oauth_verifier)
          res.write(
            'Please go back to Keybase chat to continue. You may close this page now.'
          )
          res.end()
          return
        }

        res.writeHead(404)
        res.write('not found')
        res.end()
      } catch {}
    })
    .listen(8080)

const botConfig = BotConfig.parse(process.env.JIRABOT_CONFIG || '')
if (!botConfig) {
  logger.fatal('invalid bot-config')
  process.exit(1)
} else {
  handleElbHealthCheck()
  init(botConfig).then(Bot)
}
