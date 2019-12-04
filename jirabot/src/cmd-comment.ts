import {CommentMessage} from './message'
import {Context} from './context'

const kb2jiraMention = (context: Context, kb: string) =>
  context.botConfig.jira.usernameMapper[kb] ? `[~${context.botConfig.jira.usernameMapper[kb]}]` : kb

export default (context: Context, parsedMessage: CommentMessage) =>
  context.jira
    .addComment(
      parsedMessage.ticket,
      `Comment by ${kb2jiraMention(context, parsedMessage.context.senderUsername)}: ` + parsedMessage.comment
    )
    .then(url =>
      context.bot.chat.send(parsedMessage.context.chatChannel, {
        body: `@${parsedMessage.context.senderUsername} Done! ${url}`,
      })
    )
