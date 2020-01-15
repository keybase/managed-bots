import ChatTypes from 'keybase-bot/lib/types/chat1'
import {Issue as JiraIssue} from './jira'
import {numToEmoji, statusToEmoji} from './emoji'
import {SearchMessage} from './message'
import {Context} from './context'
import * as Errors from './errors'

const issueToLine = (issue: JiraIssue, index: number) =>
  `${numToEmoji(index)} *${issue.key}* ${statusToEmoji(issue.status)} ${
    issue.summary
  } - ${issue.url}`

const buildSearchResultBody = (
  parsedMessage: SearchMessage,
  jql: string,
  issues: Array<JiraIssue>,
  additional?: string
) => {
  const begin = '```\n' + jql + '\n```\n'
  if (!issues.length) {
    return begin + 'I got nothing from Jira.'
  }
  const firstIssues = issues.slice(0, 11)
  const head =
    `@${parsedMessage.context.senderUsername} I got ${issues.length} tickets from Jira` +
    (issues.length > 11 ? '. Here are the first 11:\n\n' : ':\n\n')
  const body = firstIssues.map(issueToLine).join('\n')
  return begin + head + body + (additional ? '\n\n' + additional : '')
}

const getAssigneeAccountID = async (
  context: Context,
  parsedMessage: SearchMessage
): Promise<Errors.ResultOrError<string | undefined, Errors.UnknownError>> => {
  if (!parsedMessage.assignee) {
    return Errors.makeResult(undefined)
  }
  return Utils.getJiraAccountID(
    context,
    parsedMessage.context.teamName,
    parsedMessage.assignee
  )
}

export default async (
  context: Context,
  parsedMessage: SearchMessage,
  additional?: string
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

  const assigneeJiraRet = await getAssigneeAccountID(context, parsedMessage)
  if (assigneeJiraRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      assigneeJiraRet.error
    )
    return Errors.makeError(undefined)
  }
  const assigneeJira = assigneeJiraRet.result

  try {
    await jira
      .getOrSearch({
        query: parsedMessage.query,
        project: parsedMessage.project,
        status: parsedMessage.status,
        assigneeJira,
      })
      .then(({jql, issues}) =>
        context.bot.chat
          .send(parsedMessage.context.chatChannel, {
            body: buildSearchResultBody(parsedMessage, jql, issues, additional),
          })
          .then(({id}) => ({
            count: issues.length > 11 ? 11 : issues.length,
            id,
            issues,
          }))
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
