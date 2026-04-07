import logger from './logger'

export type BotConfig = {
  httpAddressPrefix: string // e.g. https://example.com
  keybase: {
    username: string
    paperkey: string
  }
  allowedTeams?: Array<string>
  // Optional. A list of keybase usernames that are allowed to use the `!jira
  // debug` command.
  admins?: Array<string>

  _adminsSet: Set<string>
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

  if (obj.admins) {
    if (!Array.isArray(obj.admins)) {
      logger.error('unexpect obj.admins type: not an array')
      return null
    }
    if (obj.admins.some((item: any) => typeof item !== 'string')) {
      logger.error('unexpect obj.admins: at least one item is not a string')
      return null
    }
  }
  obj._adminsSet = new Set(obj.admins || [])

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
