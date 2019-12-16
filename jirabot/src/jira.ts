import JiraClient from 'jira-connector'
import {Issue as JiraIssue} from 'jira-connector/api/issue'
import {BotConfig} from './bot-config'
import * as Configs from './configs'
import logger from './logger'
import * as Errors from './errors'
import {Context} from './context'
import mem from 'mem'

const looksLikeIssueKey = (str: string) => !!str.match(/[A-Za-z]+-[0-9]+/)

export type Issue = {
  key: string
  summary: string
  status: string
  url: string
}

class JiraClientWrapper {
  private jiraClient: JiraClient
  private jiraHost: string

  constructor(jiraClient: JiraClient, jiraHost: string) {
    this.jiraClient = jiraClient
    this.jiraHost = jiraHost
  }

  jiraRespMapper = (issue: JiraIssue): Issue => ({
    key: issue.key,
    summary: issue.fields.summary,
    status: issue.fields.status.statusCategory.name,
    url: `https://${this.jiraHost}/browse/${issue.key}`,
  })

  getOrSearch({
    query,
    project,
    status,
    assigneeJira,
  }: {
    query: string
    project: string
    status: string
    assigneeJira: string
  }): Promise<any> {
    const jql =
      (project ? `project = "${project}" AND ` : '') +
      (status ? `status = "${status}" AND ` : '') +
      (assigneeJira ? `assignee = "${assigneeJira}" AND ` : '') +
      `text ~ "${query}"`
    logger.debug({msg: 'getOrSearch', jql})
    return Promise.all([
      looksLikeIssueKey(query)
        ? this.jiraClient.issue.getIssue({
            issueKey: query,
            //fields: ['key', 'summary', 'status'],
          })
        : new Promise(r => r()),
      this.jiraClient.search.search({
        jql,
        fields: ['key', 'summary', 'status'],
        method: 'GET',
        maxResults: 11,
      }),
    ]).then(([fromGet, fromSearch]) => ({
      jql,
      issues: [
        ...(fromGet ? [fromGet] : []),
        ...(fromSearch ? fromSearch.issues : []),
      ].map(this.jiraRespMapper),
    }))
  }

  addComment(issueKey: string, comment: string): Promise<any> {
    return this.jiraClient.issue
      .addComment({
        issueKey,
        body: comment,
      })
      .then(
        ({id}: {id: string}) =>
          `https://${this.jiraHost}/browse/${issueKey}?focusedCommentId=${id}`
      )
  }

  createIssue({
    assigneeJira,
    description,
    issueType,
    name,
    project,
  }: {
    assigneeJira: string
    description: string
    issueType: string
    name: string
    project: string
  }): Promise<any> {
    logger.debug({
      msg: 'createIssue',
      assigneeJira,
      issueType,
      project,
      name,
      description,
    })
    return this.jiraClient.issue
      .createIssue({
        fields: {
          assignee: assigneeJira ? {name: assigneeJira} : undefined,
          project: {key: project.toUpperCase()},
          issuetype: {name: issueType},
          summary: name,
          description,
        },
      })
      .then(({key}: {key: string}) => `https://${this.jiraHost}/browse/${key}`)
  }
}

const jiraClientCacheTimeout = 60 * 1000 // 1min

const getJiraClient = mem(
  (
    jiraHost: string,
    accessToken: string,
    tokenSecret: string,
    consumerKey: string,
    privateKey: string
  ): JiraClient =>
    new JiraClient({
      host: jiraHost,
      oauth: {
        token: accessToken,
        token_secret: tokenSecret,
        consumer_key: consumerKey,
        private_key: privateKey,
      },
    }),
  {maxAge: jiraClientCacheTimeout, cacheKey: JSON.stringify}
)

export const getAccountId = async (
  teamJiraConfig: Configs.TeamJiraConfig,
  accessToken: string,
  tokenSecret: string
): Promise<Errors.ResultOrError<string, Errors.UnknownError>> => {
  const tempJiraClient = getJiraClient(
    teamJiraConfig.jiraHost,
    accessToken,
    tokenSecret,
    teamJiraConfig.jiraAuth.consumerKey,
    teamJiraConfig.jiraAuth.privateKey
  )
  try {
    const accountDetail = await tempJiraClient.myself.getMyself()
    return Errors.makeResult(accountDetail.accountId)
  } catch (err) {
    return Errors.makeUnknownError(err)
  }
}

export const getJiraFromTeamnameAndUsername = async (
  context: Context,
  teamname: string,
  username: string
): Promise<Errors.ResultOrError<
  JiraClientWrapper,
  Errors.JirabotNotEnabledError | Errors.UnknownError
>> => {
  const teamJiraConfigResultOrError = await context.configs.getTeamJiraConfig(
    teamname
  )
  if (teamJiraConfigResultOrError.type === Errors.ReturnType.Error) {
    switch (teamJiraConfigResultOrError.error.type) {
      case Errors.ErrorType.Unknown:
        return Errors.makeError(teamJiraConfigResultOrError.error)
      case Errors.ErrorType.KVStoreNotFound:
        return Errors.makeError(Errors.JirabotNotEnabledForTeamError)
      default:
        let _: never = teamJiraConfigResultOrError.error
        return Errors.makeError(undefined)
    }
  }
  const teamJiraConfig = teamJiraConfigResultOrError.result.config

  const teamUserConfigResultOrError = await context.configs.getTeamUserConfig(
    teamname,
    username
  )
  if (teamUserConfigResultOrError.type === Errors.ReturnType.Error) {
    switch (teamUserConfigResultOrError.error.type) {
      case Errors.ErrorType.Unknown:
        return Errors.makeError(teamUserConfigResultOrError.error)
      case Errors.ErrorType.KVStoreNotFound:
        return Errors.makeError(Errors.JirabotNotEnabledForUserError)
      default:
        let _: never = teamUserConfigResultOrError.error
        return Errors.makeError(undefined)
    }
  }
  const teamUserConfig = teamUserConfigResultOrError.result.config

  const jiraClient = getJiraClient(
    teamJiraConfig.jiraHost,
    teamUserConfig.accessToken,
    teamUserConfig.tokenSecret,
    teamJiraConfig.jiraAuth.consumerKey,
    teamJiraConfig.jiraAuth.privateKey
  )

  return Errors.makeResult(
    new JiraClientWrapper(jiraClient, teamJiraConfig.jiraHost)
  )
}
