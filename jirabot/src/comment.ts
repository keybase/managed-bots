import * as ChatTypes from 'keybase-bot/lib/types/chat1'
import {CommentMessage} from './message'
import {Context} from './context'

const kb2jiraMention = (context: Context, kb: string) =>
  context.config.jira.usernameMapper[kb] ? `[~${context.config.jira.usernameMapper[kb]}]` : kb

export default (context: Context, channel: ChatTypes.ChatChannel, parsedMessage: CommentMessage) =>
  context.jira
    .addComment(parsedMessage.ticket, `Comment by ${kb2jiraMention(context, parsedMessage.from)}: ` + parsedMessage.comment)
    .then(url =>
      context.bot.chat.send(channel, {
        body: `@${parsedMessage.from} Done! ${url}`,
      })
    )
