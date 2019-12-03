export type BotConfig = {
  keybase: {
    username: string
    paperkey: string
  }
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
    console.error('unexpect obj type', typeof obj)
    return null
  }

  if (typeof obj.keybase !== 'object') {
    console.error('unexpect obj.keybase type', typeof obj.keybase)
    return null
  }
  if (typeof obj.keybase.username !== 'string') {
    console.error('unexpect obj.keybase.username type', typeof obj.keybase.username)
    return null
  }
  if (typeof obj.keybase.paperkey !== 'string') {
    console.error('unexpect obj.keybase.paperkey type', typeof obj.keybase.paperkey)
    return null
  }

  if (typeof obj.jira !== 'object') {
    console.error('unexpect obj.jira type', typeof obj.jira)
    return null
  }
  if (typeof obj.jira.host !== 'string') {
    console.error('unexpect obj.jira.host type', typeof obj.jira.host)
    return null
  }
  if (typeof obj.jira.email !== 'string') {
    console.error('unexpect obj.jira.email type', typeof obj.jira.email)
    return null
  }
  if (typeof obj.jira.apiToken !== 'string') {
    console.error('unexpect obj.jira.apiToken type', typeof obj.jira.apiToken)
    return null
  }
  if (!Array.isArray(obj.jira.projects)) {
    console.error('unexpect obj.jira.projects type: not an array', obj.jira.projects)
    return null
  }
  if (!Array.isArray(obj.jira.issueTypes)) {
    console.error('unexpect obj.jira.issueTypes type: not an array', obj.jira.issueTypes)
    return null
  }
  if (!Array.isArray(obj.jira.status)) {
    console.error('unexpect obj.jira.status type: not an array', obj.jira.status)
    return null
  }

  // case-insensitive
  obj.jira.projects = obj.jira.projects.map((project: string) => project.toLowerCase())
  obj.jira.status = obj.jira.status.map((status: string) => status.toLowerCase())

  obj.jira._issueTypeInsensitiveToIssueType = (() => {
    const mapper = new Map(obj.jira.issueTypes.map((original: string) => [original.toLowerCase(), original]))
    return (issueType: string) => mapper.get(issueType.toLowerCase()) || issueType
  })()

  // TODO validate usernameMapper maybe

  return obj as BotConfig
}

export const parse = (base64BotConfig: string): null | BotConfig => {
  try {
    return checkBotConfig(JSON.parse(Buffer.from(base64BotConfig, 'base64').toString()))
  } catch (e) {
    console.error(e)
    return null
  }
}
