import Bot from 'keybase-bot'
import {Issue} from './jira'
import {CommentMessage} from './message'
import util from 'util'
import * as BotConfig from './bot-config'
import * as Jira from './jira'
import Aliases from './aliases'
import Configs from './configs'
import StatHat from './stathat'
import logger from './logger'

const setTimeoutPromise = util.promisify(setTimeout)

type CommentContextItem = {
  message: CommentMessage
  issues: Array<Issue>
}

class CommentContext {
  _respMsgIDToCommentMessage = new Map()

  add = (responseID: number, message: CommentMessage, issues: Array<Issue>) => {
    this._respMsgIDToCommentMessage.set(responseID, {message, issues})
    setTimeoutPromise(1000 * 120 /* 2min */).then(() =>
      this._respMsgIDToCommentMessage.delete(responseID)
    )
  }

  get = (responseID: number): null | CommentContextItem =>
    this._respMsgIDToCommentMessage.get(responseID)
}

export type Context = {
  aliases: Aliases
  bot: Bot
  botConfig: BotConfig.BotConfig
  comment: CommentContext
  configs: Configs
  getJiraFromTeamnameAndUsername: typeof Jira.getJiraFromTeamnameAndUsername
  stathat: StatHat
}

const logSendAndExit = async (context: Context): Promise<void> => {
  logger.info({
    msg: 'calling logSend',
  })
  try {
    await context.bot.logSend()
    logger.info({
      msg: 'logSend succeeded',
    })
    process.exit(0)
  } catch (err) {
    logger.warn({
      msg: 'logSend failed',
      err,
    })
    process.exit(1)
  }
}

export const init = async (
  botConfig: BotConfig.BotConfig
): Promise<Context> => {
  var bot = new Bot({debugLogging: true})
  const context = {
    aliases: new Aliases({}),
    bot,
    botConfig,
    comment: new CommentContext(),
    configs: new Configs(bot, botConfig),
    getJiraFromTeamnameAndUsername: Jira.getJiraFromTeamnameAndUsername,
    stathat: new StatHat(botConfig),
  }
  await context.bot.init(
    context.botConfig.keybase.username,
    context.botConfig.keybase.paperkey,
    {
      verbose: true,
      autoLogSendOnCrash: true,
    }
  )
  logger.info('init done')
  process.on('SIGTERM', () => logSendAndExit(context))
  return context
}
