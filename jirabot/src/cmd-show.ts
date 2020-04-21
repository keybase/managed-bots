import {Issue as JiraIssue} from './jira'
import {numToEmoji, statusToEmoji} from './emoji'
import {ShowMessage} from './message'
import {Context} from './context'
import * as Errors from './errors'
import * as Utils from './utils'

const issueTypeToEmojiMaybe = (issueType: string) => {
  switch (issueType) {
    case 'Story':
      return ':scroll:'
    case 'Bug':
      return ':bug:'
    case 'Epic':
      return ':six_pointed_star:'
    case 'Task':
      return ':ballot_box_with_check:'
    case 'Subtask':
    default:
      return `[${issueType}]`
  }
}

const formatIssue = (issueKeyFromUser: string, issue: JiraIssue) => {
  console.log(JSON.stringify(issue, null, 2))

  const title = `${issueTypeToEmojiMaybe(issue.issueType)} *${issue.key}*${
    issueKeyFromUser.toLowerCase() === issue.key.toLowerCase()
      ? ''
      : ` (${issueKeyFromUser})`
  } | ${issue.status} ${issue.url}`
  const summary = `*${issue.summary}*`
  const metadata = `Reported by _${issue.reporterJira}_ ${
    issue.createdTimeHumanized
  }. ${
    issue.assigneeJira
      ? `Assigned to _${issue.assigneeJira}_.`
      : 'Not assigned.'
  }`

  return [title, summary, metadata].join('\n')
}

export default async (
  context: Context,
  parsedMessage: ShowMessage
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
    const issues = await Promise.all(
      parsedMessage.issueKeys.map(issueKey =>
        jira
          .get({
            issueKey,
          })
          .then((issue: JiraIssue) => ({issue, issueKey}))
          .catch(error => ({error, issueKey}))
      )
    )

    issues.forEach(({error, issue, issueKey}) =>
      issue
        ? Utils.replyToMessageContext(
            context,
            parsedMessage.context,
            formatIssue(issueKey, issue),
            issues.length === 1
          )
        : Utils.replyToMessageContext(
            context,
            parsedMessage.context,
            `:warning: Failed to get issue: ${issueKey}`,
            issues.length === 1
          )
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
