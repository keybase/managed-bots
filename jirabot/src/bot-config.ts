import logger from './logger'

export type BotConfig = {
  httpAddressPrefix: string // e.g. https://example.com
  keybase: {
    username: string
    paperkey: string
  }
  allowedTeams: Array<string>
  // TODO: move this to TeamConfig
  jira: {
    host: string
    email: string
    apiToken: string
    projects: Array<string>
    issueTypes: Array<string>
    status: Array<string>
    usernameMapper: {
      [key: string]: string
    }

    _issueTypeInsensitiveToIssueType: (issueType: string) => string
  }
}

const checkBotConfig = (obj: any): null | BotConfig => {
  if (typeof obj !== 'object') {
    logger.error('unexpect obj type', typeof obj)
    return null
  }

  if (typeof obj.httpAddressPrefix !== 'string') {
    logger.error(
      'unexpect obj.httpAddressPrefix type',
      typeof obj.httpAddressPrefix
    )
    return null
  }
  if (obj.httpAddressPrefix.endsWith('/')) {
    obj.httpAddressPrefix = obj.httpAddressPrefix.slice(
      0,
      obj.httpAddressPrefix.length - 1
    )
  }

  if (typeof obj.keybase !== 'object') {
    logger.error('unexpect obj.keybase type', typeof obj.keybase)
    return null
  }
  if (typeof obj.keybase.username !== 'string') {
    logger.error(
      'unexpect obj.keybase.username type',
      typeof obj.keybase.username
    )
    return null
  }
  if (typeof obj.keybase.paperkey !== 'string') {
    logger.error(
      'unexpect obj.keybase.paperkey type',
      typeof obj.keybase.paperkey
    )
    return null
  }

  if (obj.allowedTeams && !Array.isArray(obj.allowedTeams)) {
    logger.error(
      'unexpect obj.allowedTeam type: not an array',
      obj.allowedTeams
    )
    return null
  }

  if (typeof obj.jira !== 'object') {
    logger.error('unexpect obj.jira type', typeof obj.jira)
    return null
  }
  if (typeof obj.jira.host !== 'string') {
    logger.error('unexpect obj.jira.host type', typeof obj.jira.host)
    return null
  }
  if (typeof obj.jira.email !== 'string') {
    logger.error('unexpect obj.jira.email type', typeof obj.jira.email)
    return null
  }
  if (typeof obj.jira.apiToken !== 'string') {
    logger.error('unexpect obj.jira.apiToken type', typeof obj.jira.apiToken)
    return null
  }
  if (!Array.isArray(obj.jira.projects)) {
    logger.error(
      'unexpect obj.jira.projects type: not an array',
      obj.jira.projects
    )
    return null
  }
  if (!Array.isArray(obj.jira.issueTypes)) {
    logger.error(
      'unexpect obj.jira.issueTypes type: not an array',
      obj.jira.issueTypes
    )
    return null
  }
  if (!Array.isArray(obj.jira.status)) {
    logger.error('unexpect obj.jira.status type: not an array', obj.jira.status)
    return null
  }

  // case-insensitive
  obj.jira.projects = obj.jira.projects.map((project: string) =>
    project.toLowerCase()
  )
  obj.jira.status = obj.jira.status.map((status: string) =>
    status.toLowerCase()
  )

  obj.jira._issueTypeInsensitiveToIssueType = (() => {
    const mapper = new Map(
      obj.jira.issueTypes.map((original: string) => [
        original.toLowerCase(),
        original,
      ])
    )
    return (issueType: string) =>
      mapper.get(issueType.toLowerCase()) || issueType
  })()

  // TODO validate usernameMapper maybe

  return obj as BotConfig
}

export const parse = (base64BotConfig: string): null | BotConfig => {
  try {
    return checkBotConfig(
      JSON.parse(Buffer.from(base64BotConfig, 'base64').toString())
    )
  } catch (e) {
    logger.error(e)
    return null
  }
}
