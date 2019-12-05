import JiraClient from 'jira-connector'
import {Issue as JiraIssue} from 'jira-connector/api/issue'
import {BotConfig} from './bot-config'
import logger from './logger'

const looksLikeIssueKey = (str: string) => !!str.match(/[A-Za-z]+-[0-9]+/)

export type Issue = {
  key: string
  summary: string
  status: string
  url: string
}

export default class {
  _botConfig: BotConfig
  _jira: JiraClient

  constructor(botConfig: BotConfig) {
    this._botConfig = botConfig
    this._jira = new JiraClient({
      host: botConfig.jira.host,
      basic_auth: {
        email: botConfig.jira.email,
        api_token: botConfig.jira.apiToken,
      },
    })
  }

  jiraRespMapper = (issue: JiraIssue): Issue => ({
    key: issue.key,
    summary: issue.fields.summary,
    status: issue.fields.status.statusCategory.name,
    url: `https://${this._botConfig.jira.host}/browse/${issue.key}`,
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
        ? this._jira.issue.getIssue({
            issueKey: query,
            //fields: ['key', 'summary', 'status'],
          })
        : new Promise(r => r()),
      this._jira.search.search({
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
    return this._jira.issue
      .addComment({
        issueKey,
        comment: {body: comment},
      })
      .then(
        ({id}: {id: string}) =>
          `https://${this._botConfig.jira.host}/browse/${issueKey}?focusedCommentId=${id}`
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
    return this._jira.issue
      .createIssue({
        fields: {
          assignee: assigneeJira ? {name: assigneeJira} : undefined,
          project: {key: project.toUpperCase()},
          issuetype: {name: issueType},
          summary: name,
          description,
        },
      })
      .then(
        ({key}: {key: string}) =>
          `https://${this._botConfig.jira.host}/browse/${key}`
      )
  }
}
