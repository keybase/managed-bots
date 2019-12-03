import {CreateMessage} from './message'
import {Context} from './context'

export default (context: Context, parsedMessage: CreateMessage) =>
  context.jira
    .createIssue({
      assigneeJira: context.botConfig.jira.usernameMapper[parsedMessage.assignee] || '',
      project: parsedMessage.project,
      name: parsedMessage.name,
      description:
        `Reported by [~${context.botConfig.jira.usernameMapper[parsedMessage.context.senderUsername]}]: \n` + parsedMessage.description,
      issueType: parsedMessage.issueType || context.botConfig.jira.issueTypes[0],
    })
    .then((url: string) =>
      context.bot.chat.send(parsedMessage.context.chatChannel, {
        body: 'Ticket created' + (parsedMessage.assignee ? ` for @${parsedMessage.assignee}` : '') + `: ${url}`,
      })
    )
