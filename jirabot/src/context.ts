import Bot from 'keybase-bot'
import {Issue} from './jira'
import {CommentMessage} from './message'
import util from 'util'
import * as Config from './config'
import Jira from './jira'
import Aliases from './aliases'

const setTimeoutPromise = util.promisify(setTimeout)

type CommentContextItem = {
  message: CommentMessage
  issues: Array<Issue>
}

class CommentContext {
  _respMsgIDToCommentMessage = new Map()

  add = (responseID: number, message: CommentMessage, issues: Array<Issue>) => {
    this._respMsgIDToCommentMessage.set(responseID, {message, issues})
    setTimeoutPromise(1000 * 120 /* 2min */).then(() => this._respMsgIDToCommentMessage.delete(responseID))
  }

  get = (responseID: number): null | CommentContextItem => this._respMsgIDToCommentMessage.get(responseID)
}

export type Context = {
  aliases: Aliases
  bot: Bot
  config: Config.Config
  comment: CommentContext
  jira: Jira
}

export const init = (config: Config.Config): Promise<Context> => {
  const context = {
    aliases: new Aliases({}),
    bot: new Bot(),
    config,
    comment: new CommentContext(),
    jira: new Jira(config),
  }
  return context.bot
    .init(context.config.keybase.username, context.config.keybase.paperkey, {
      verbose: true,
    })
    .then(() => {
      console.debug({msg: 'init done'})
      return context
    })
}
