import http from 'http'
import url from 'url'
import {onJiraCallback} from './jira-oauth'
import * as Constants from './constants'
import {Context} from './context'
import * as Errors from './errors'
import handleWebhookEvent from './handle-webhook-event'
import logger from './logger'

const healthCheck = async (
  context: Context,
  _: http.IncomingMessage,
  res: http.ServerResponse
): Promise<void> => {
  try {
    await context.bot.helpers.ping()
    res.write('ok')
    res.end()
  } catch (err) {
    res.writeHead(500)
    res.write(err.toString())
    res.end()
  }
}

const jiraOauthCallback = (
  parsedUrl: url.UrlWithParsedQuery,
  res: http.ServerResponse
) => {
  const {oauth_token, oauth_verifier} = parsedUrl.query
  if (typeof oauth_token !== 'string' || typeof oauth_verifier !== 'string') {
    res.writeHead(400)
    res.end('unexpected callback data')
    return
  }
  onJiraCallback(oauth_token, oauth_verifier)
  res.write(
    'Please go back to Keybase chat to continue. You may close this page now.'
  )
  res.end()
}

const readAll = (req: http.IncomingMessage): Promise<string> =>
  new Promise((resolve, reject) => {
    let body = ''
    req.on('data', chunk => (body += chunk))
    req.on('end', () => resolve(body))
    req.on('error', () => reject(body))
  })

const jiraWebhook = async (
  context: Context,
  parsedUrl: url.UrlWithParsedQuery,
  req: http.IncomingMessage,
  res: http.ServerResponse
) => {
  const {urlToken} = parsedUrl.query
  if (typeof urlToken !== 'string') {
    logger.warn({msg: 'jiraWebhook', error: 'urlToken !== string'})
    res.writeHead(400)
    res.end('unexpected callback data')
    return
  }

  const fromIndexRet = await context.configs.getJiraSubscriptionIndex(urlToken)
  if (fromIndexRet.type === Errors.ReturnType.Error) {
    if (fromIndexRet.error.type === Errors.ErrorType.KVStoreNotFound) {
      logger.warn({msg: 'jiraWebhook', error: 'subscription index not found'})
      res.writeHead(400)
      res.end('unexpected callback data')
      return
    } else {
      logger.warn({msg: 'jiraWebhook', error: fromIndexRet.error})
      res.writeHead(500)
      res.end()
      return
    }
  }
  const fromIndex = fromIndexRet.result.config

  const getSubRet = await context.configs.getTeamJiraSubscriptions(
    fromIndex.teamname
  )
  if (getSubRet.type === Errors.ReturnType.Error) {
    if (getSubRet.error.type === Errors.ErrorType.KVStoreNotFound) {
      logger.warn({msg: 'jiraWebhook', error: 'subscription not found'})
      res.writeHead(400)
      res.end('unexpected callback data')
      return
    } else {
      logger.warn({msg: 'jiraWebhook', error: getSubRet.error})
      res.writeHead(500)
      res.end()
      return
    }
  }
  const subscriptions = getSubRet.result.config

  const subscription = subscriptions.get(fromIndex.id)
  if (!subscription) {
    logger.warn({msg: 'jiraWebhook', error: 'subscription ID not found'})
    res.writeHead(400)
    res.end('unexpected callback data')
    return
  }

  const json = await readAll(req)
  let payload = undefined
  try {
    payload = JSON.parse(json)
  } catch (error) {
    logger.warn({msg: 'jiraWebhook', error})
    res.writeHead(400)
    res.end('unexpected callback data')
    return
  }

  await handleWebhookEvent(context, fromIndex.teamname, subscription, payload)

  // If it's a legit payload, just return 200 no matter we can handle it or
  // not. This way Jira won't retry the same notification.
  res.writeHead(200)
  res.end()
}

export default (context: Context) =>
  http
    .createServer((req, res) => {
      try {
        if (req.url === Constants.healthCheckPathname) {
        }

        const parsedUrl = url.parse(req.url, true)
        switch (parsedUrl.pathname) {
          case Constants.healthCheckPathname:
            healthCheck(context, req, res)
            return
          case Constants.jiraOauthCallbackPathname:
            jiraOauthCallback(parsedUrl, res)
            return
          case Constants.jiraWebhookPathname:
            jiraWebhook(context, parsedUrl, req, res)
            return
          default:
            res.writeHead(404)
            res.write('not found')
            res.end()
            return
        }
      } catch {}
    })
    .listen(8080)
