import {CommentMessage} from './message'
import {Context} from './context'
import * as Errors from './errors'
import * as Utils from './utils'

const kb2jiraMention = (context: Context, kb: string) =>
  context.botConfig.jira.usernameMapper[kb]
    ? `[~${context.botConfig.jira.usernameMapper[kb]}]`
    : kb

export default async (
  context: Context,
  parsedMessage: CommentMessage
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
    const url = await jira.addComment(
      parsedMessage.ticket,
      parsedMessage.comment
    )
    await Utils.replyToMessageContext(
      context,
      parsedMessage.context,
      `@${parsedMessage.context.senderUsername} Done! ${url}`
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
