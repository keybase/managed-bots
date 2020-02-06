import {Context} from './context'
import * as Configs from './configs'
import * as Jira from './jira'
import * as Errors from './errors'
import logger from './logger'

type Issue = {
  type: string
  issueKey: string
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
        issueKey,
        url: `https://${jiraHost}/browse/${issueKey}`,
        reporter,
        project,
        summary,
      }
    : undefined
}

enum ChangelogType {
  Assignee = 'assignee',
  Status = 'status',
  Points = 'Story Points',
  Sprint = 'Sprint',
  Summary = 'summary',
  Project = 'project',
}

type ChangelogItemAssignee = {
  type: ChangelogType.Assignee
  from?: string
  to?: string
}

type ChangelogItemStatus = {
  type: ChangelogType.Status
  from?: string
  to?: string
}

type ChangelogItemPoints = {
  type: ChangelogType.Points
  from?: string
  to?: string
}

type ChangelogItemSprint = {
  type: ChangelogType.Sprint
  from?: string
  to?: string
}

type ChangelogItemSummary = {
  type: ChangelogType.Summary
  from?: string
  to?: string
}

type ChangelogItemProject = {
  type: ChangelogType.Project
  from?: string
  to?: string
}

type ChangelogItemNoProject =
  | ChangelogItemAssignee
  | ChangelogItemStatus
  | ChangelogItemPoints
  | ChangelogItemSprint
  | ChangelogItemSummary

const supportedChangelogTypeForUpdates = new Set([
  ChangelogType.Assignee,
  ChangelogType.Status,
  ChangelogType.Points,
  ChangelogType.Sprint,
  ChangelogType.Summary,
  // We deal with ChangelogType.Project separrately.
])

const parseChangelogForUpdates = (
  changelog: any
): Array<ChangelogItemNoProject> =>
  (Array.isArray(changelog.items) ? changelog.items : []).reduce(
    (items: Array<ChangelogItemNoProject>, item: any) => {
      if (!supportedChangelogTypeForUpdates.has(item.field)) {
        return items
      }
      const from = item.fromString || undefined
      const to = item.toString || undefined
      return !from && !to
        ? items
        : [
            ...items,
            {
              type: item.field,
              from,
              to,
            },
          ]
    },
    [] as Array<ChangelogItemNoProject>
  )

const parseChangelogForProjectUpdate = (
  changelog: any
): undefined | ChangelogItemProject => {
  const entry = (Array.isArray(changelog.items) ? changelog.items : []).find(
    (entry: any) => entry.field === ChangelogType.Project
  )
  if (!entry || !entry.fromString || !entry.toString) {
    return undefined
  }
  return {
    type: ChangelogType.Project,
    from: entry.fromString,
    to: entry.toString,
  }
}

export default async (
  context: Context,
  teamname: string,
  subscription: Configs.TeamJiraSubscription,
  payload: any
): Promise<any> => {
  if (typeof payload !== 'object') {
    logger.warn({
      msg: 'handleWebhookEvent',
      error: 'unexpected payload',
      payload,
    })
    return undefined
  }

  const {webhookEvent} = payload

  if (
    webhookEvent !== Jira.JiraSubscriptionEvents.IssueCreated &&
    webhookEvent !== Jira.JiraSubscriptionEvents.IssueUpdated
  ) {
    logger.warn({
      msg: 'handleWebhookEvent',
      error: 'unknown webhook event',
    })
    return undefined
  }

  const teamJiraConfigRet = await context.configs.getTeamJiraConfig(teamname)
  if (teamJiraConfigRet.type === Errors.ReturnType.Error) {
    logger.warn({msg: 'handleWebhookEvent', error: teamJiraConfigRet.error})
    return undefined
  }
  const teamJiraConfig = teamJiraConfigRet.result.config

  const issue =
    payload.issue &&
    parseIssueFromPayload(payload.issue, teamJiraConfig.jiraHost)
  if (!issue) {
    logger.warn({
      msg: 'handleWebhookEvent',
      error: 'unexpected issue',
      issue,
    })
    return undefined
  }

  switch (webhookEvent) {
    case Jira.JiraSubscriptionEvents.IssueCreated:
      context.bot.chat.send(subscription.conversationId, {
        body: `${issue.reporter} reported a new _${issue.type}_ in ${issue.project}: *${issue.summary}*\n${issue.url}`,
      })
      return undefined
    case Jira.JiraSubscriptionEvents.IssueUpdated:
      const projectUpdate = parseChangelogForProjectUpdate(payload.changelog)
      projectUpdate &&
        context.bot.chat.send(subscription.conversationId, {
          body: `A _${issue.type}_ was moved from ~_${projectUpdate.from}_~ to *${projectUpdate.to}*: ${issue.summary} | ${issue.url}`,
        })

      if (!subscription.withUpdates) {
        return undefined
      }

      const changelogItems =
        payload.changelog && parseChangelogForUpdates(payload.changelog)
      if (!changelogItems.length) {
        logger.warn({
          msg: 'handleWebhookEvent',
          error: 'empty changelog',
          changelogItems,
        })
        return undefined
      }
      context.bot.chat.send(subscription.conversationId, {
        body:
          `Updated: [${issue.type}] ${issue.summary} | ${issue.url}\n` +
          changelogItems
            .map(item => {
              switch (item.type) {
                case ChangelogType.Assignee:
                  if (item.from && item.to) {
                    return `Reassigned from ~_${item.from}_~ to *${item.to}*.`
                  } else if (item.from) {
                    return `Assignee ~_${item.from}_~ is removed.`
                  } else if (item.to) {
                    return `Assigned to *${item.to}*.`
                  }
                  return ''
                case ChangelogType.Status:
                  return `Moved from ~_${item.from}_~ to *${item.to}*.`
                case ChangelogType.Points:
                  if (item.from && item.to) {
                    return `Story Points is changed from ~_${item.from}_~ to *${item.to}*.`
                  } else if (item.from) {
                    return `Story Points ~_${item.from}_~ is removed.`
                  } else if (item.to) {
                    return `Story Points is added: *${item.to}*.`
                  }
                  return ''
                case ChangelogType.Sprint:
                  if (item.from && item.to) {
                    return `Moved from ~_${item.from}_~ to *${item.to}*.`
                  } else if (item.from) {
                    return `Removed from ~_${item.from}_~.`
                  } else if (item.to) {
                    return `Added into *${item.to}*.`
                  }
                  return ''
                case ChangelogType.Summary:
                  if (item.from) {
                    return `Reworded from ~_${item.from}_~ to *${item.to}*.`
                  }
                  return ''
              }
            })
            .filter(Boolean)
            .map(line => '> ' + line)
            .join('\n'),
      })
      return undefined
  }
}
