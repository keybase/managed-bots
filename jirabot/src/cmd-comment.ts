import {CommentMessage} from './message'
import {Context} from './context'
import * as Errors from './errors'

const kb2jiraMention = (context: Context, kb: string) =>
  context.botConfig.jira.usernameMapper[kb]
    ? `[~${context.botConfig.jira.usernameMapper[kb]}]`
    : kb

export default async (
  context: Context,
  parsedMessage: CommentMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  const jiraResultOrError = await context.getJiraFromTeamnameAndUsername(
    context,
    parsedMessage.context.teamName,
    parsedMessage.context.senderUsername
  )
  if (jiraResultOrError.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      jiraResultOrError.error
    )
    return Errors.makeError(undefined)
  }
  const jira = jiraResultOrError.result
  try {
    const url = await jira.addComment(
      parsedMessage.ticket,
      parsedMessage.comment
    )
    await context.bot.chat.send(parsedMessage.context.chatChannel, {
      body: `@${parsedMessage.context.senderUsername} Done! ${url}`,
    })
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
