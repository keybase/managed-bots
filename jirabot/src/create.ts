import ChatTypes from 'keybase-bot/lib/types/chat1'
import {CreateMessage} from './message'
import {Context} from './context'

export default (context: Context, channel: ChatTypes.ChatChannel, parsedMessage: CreateMessage) =>
  context.jira
    .createIssue({
      assigneeJira: context.config.jira.usernameMapper[parsedMessage.assignee] || '',
      project: parsedMessage.project,
      name: parsedMessage.name,
      description: `Reported by [~${context.config.jira.usernameMapper[parsedMessage.from]}]: \n` + parsedMessage.description,
      issueType: parsedMessage.issueType || context.config.jira.issueTypes[0],
    })
    .then((url: string) =>
      context.bot.chat.send(channel, {
        body: 'Ticket created' + (parsedMessage.assignee ? ` for @${parsedMessage.assignee}` : '') + `: ${url}`,
      })
    )
