import {CreateMessage} from './message'
import {Context} from './context'
import * as Errors from './errors'
import * as Utils from './utils'

export default async (
  context: Context,
  parsedMessage: CreateMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  const jiraRet = await context.getJiraFromTeamnameAndUsername(
    context,
    parsedMessage.context.teamName,
    parsedMessage.context.senderUsername
  )
  if (jiraRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      jiraRet.error
    )
    return Errors.makeError(undefined)
  }
  const jira = jiraRet.result
  try {
    const url = await jira.createIssue({
      assigneeJira:
        context.botConfig.jira.usernameMapper[parsedMessage.assignee] || '',
      project: parsedMessage.project,
      name: parsedMessage.name,
      description: parsedMessage.description,
      issueType:
        parsedMessage.issueType || context.botConfig.jira.issueTypes[0],
    })
    await Utils.replyToMessageContext(
      context,
      parsedMessage.context,
      'Ticket created' +
        (parsedMessage.assignee ? ` for @${parsedMessage.assignee}` : '') +
        `: ${url}`
    )
    return Errors.makeResult(undefined)
  } catch (err) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      Errors.makeUnknownError(err).error
    )
    return Errors.makeError(undefined)
  }
}
