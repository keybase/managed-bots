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

  getIssueTypes(): Promise<Array<string>> {
    logger.debug({
      msg: 'getIssueTypes',
    })
    return this.jiraClient.issueType
      .getAllIssueTypes()
      .then((resp: Array<{name: string}>) => resp.map(({name}) => name))
  }

  getProjects(): Promise<Array<string>> {
    logger.debug({
      msg: 'getProjects',
    })
    return this.jiraClient.project
      .getAllProjects()
      .then((resp: Array<{key: string}>) => resp.map(({key}) => key))
  }

  getStatuses(): Promise<Array<string>> {
    logger.debug({
      msg: 'getStatuses',
    })
    return this.jiraClient.status
      .getAllStatuses()
      .then((resp: Array<{name: string}>) => resp.map(({name}) => name))
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
  const teamJiraConfigRet = await context.configs.getTeamJiraConfig(teamname)
  if (teamJiraConfigRet.type === Errors.ReturnType.Error) {
    switch (teamJiraConfigRet.error.type) {
      case Errors.ErrorType.Unknown:
        return Errors.makeError(teamJiraConfigRet.error)
      case Errors.ErrorType.KVStoreNotFound:
        return Errors.makeError(Errors.JirabotNotEnabledForTeamError)
      default:
        let _: never = teamJiraConfigRet.error
        return Errors.makeError(undefined)
    }
  }
  const teamJiraConfig = teamJiraConfigRet.result.config

  const teamUserConfigRet = await context.configs.getTeamUserConfig(
    teamname,
    username
  )
  if (teamUserConfigRet.type === Errors.ReturnType.Error) {
    switch (teamUserConfigRet.error.type) {
      case Errors.ErrorType.Unknown:
        return Errors.makeError(teamUserConfigRet.error)
      case Errors.ErrorType.KVStoreNotFound:
        return Errors.makeError(Errors.JirabotNotEnabledForUserError)
      default:
        let _: never = teamUserConfigRet.error
        return Errors.makeError(undefined)
    }
  }
  const teamUserConfig = teamUserConfigRet.result.config

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

class JiraMetadata {
  private issueTypesArray: Array<string>
  private projectsArray: Array<string>
  private statusesArray: Array<string>

  // lowercased => original
  private issueTypesSet: Map<string, string>
  private projectsSet: Map<string, string>
  private statusesSet: Map<string, string>

  constructor(data: {
    issueTypes: Array<string>
    projects: Array<string>
    statuses: Array<string>
  }) {
    this.issueTypesArray = [...data.issueTypes]
    this.projectsArray = [...data.projects]
    this.statusesArray = [...data.statuses]
    this.issueTypesSet = new Map(data.issueTypes.map(s => [s.toLowerCase(), s]))
    this.projectsSet = new Map(data.projects.map(s => [s.toLowerCase(), s]))
    this.statusesSet = new Map(data.statuses.map(s => [s.toLowerCase(), s]))
  }

  normalizeIssueType(issueType: string): undefined | string {
    return this.issueTypesSet.get(issueType.toLowerCase())
  }
  normalizeProject(projectKey: string): undefined | string {
    return this.projectsSet.get(projectKey.toLowerCase())
  }
  normalizeStatus(status: string): undefined | string {
    return this.statusesSet.get(status.toLowerCase())
  }

  issueTypes(): Array<string> {
    return [...this.issueTypesArray]
  }
  projects(): Array<string> {
    return [...this.projectsArray]
  }
  statuses(): Array<string> {
    return [...this.statusesArray]
  }
}

export const getJiraMetadata = async (
  context: Context,
  teamname: string,
  username: string
): Promise<Errors.ResultOrError<
  JiraMetadata,
  Errors.UnknownError | Errors.JirabotNotEnabledError
>> => {
  // TODO cache

  const jiraRet = await getJiraFromTeamnameAndUsername(
    context,
    teamname,
    username
  )
  if (jiraRet.type === Errors.ReturnType.Error) {
    switch (jiraRet.error.type) {
      case Errors.ErrorType.JirabotNotEnabled:
        break
      case Errors.ErrorType.Unknown:
        logger.warn({msg: 'getJiraMetadata', error: jiraRet.error})
        break
      default:
        let _: never = jiraRet.error
    }
    return jiraRet
  }

  const jira = jiraRet.result
  try {
    const issueTypes = await jira.getIssueTypes()
    const projects = await jira.getProjects()
    const statuses = await jira.getStatuses()
    return Errors.makeResult(new JiraMetadata({issueTypes, projects, statuses}))
  } catch (error) {
    return Errors.makeUnknownError(error)
  }
}
