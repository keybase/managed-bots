import {Context} from './context'
import * as Configs from './configs'
import * as Jira from './jira'
import * as Errors from './errors'
import * as Utils from './utils'

type Issue = {
  type: string
  url: string
  reporter: string
  project: string
  summary: string
}

const parseIssueFromPayload = (
  issue: any,
  jiraHost: string
): undefined | Issue => {
  const type = issue?.fields?.issuetype?.name
  const issueKey = issue?.key
  const reporter = issue?.fields?.reporter?.displayName
  const project = issue?.fields?.project?.name
  const summary = issue?.fields?.summary
  return type && issueKey && reporter && project && summary
    ? {
        type,
        url: `https://${jiraHost}/browse/${issueKey}`,
        reporter,
        project,
        summary,
      }
    : undefined
}

export default async (
  context: Context,
  teamname: string,
  subscription: Configs.TeamJiraSubscription,
  payload: any
): Promise<any> => {
  console.log({songgao: 'handleWebhookEvent', payload})
  if (typeof payload !== 'object') {
    console.log({
      songgao: 'handleWebhookEvent not object',
      type: typeof payload,
    })
    return undefined
  }
  switch (payload.webhookEvent) {
    case Jira.JiraSubscriptionEvents.IssueCreated:
      const teamJiraConfigRet = await context.configs.getTeamJiraConfig(
        teamname
      )
      if (teamJiraConfigRet.type === Errors.ReturnType.Error) {
        logger.warn({msg: 'handleWebhookEvent', error: teamJiraConfigRet.error})
        return undefined
      }
      const teamJiraConfig = teamJiraConfigRet.result.config

      const issue =
        payload.issue &&
        parseIssueFromPayload(payload.issue, teamJiraConfig.jiraHost)
      if (!issue) {
        console.log({songgao: 'handleWebhookEvent no issue'})
        return undefined
      }
      context.bot.chat.send(subscription.conversationId, {
        body: `${issue.reporter} created a new ${issue.type} in ${issue.project}: ${issue.summary}\n${issue.url}`,
      })
      return undefined
    case Jira.JiraSubscriptionEvents.IssueUpdated:
    // TODO
    default:
      console.log({
        songgao: 'handleWebhookEvent default',
        webhookEvent: payload.webhookEvent,
      })
      return undefined
  }
}
