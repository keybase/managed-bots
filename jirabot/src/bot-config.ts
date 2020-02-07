import logger from './logger'

export type BotConfig = {
  httpAddressPrefix: string // e.g. https://example.com
  keybase: {
    username: string
    paperkey: string
  }
  stathat?: {
    ezkey: string
    // Optional. Stat names are constructed by directly concatenating this
    // prefix and individual stat names.
    // Example: 'jirabot-prod - '.
    prefix: string
  }
  allowedTeams?: Array<string>
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

  if (obj.stathat) {
    if (typeof obj.stathat !== 'object') {
      logger.error('unexpect obj.stathat type', typeof obj.stathat)
      return null
    }
    if (typeof obj.stathat.ezkey !== 'string') {
      logger.error('unexpect obj.stathat.ezkey type', typeof obj.stathat.ezkey)
      return null
    }
    if (!obj.stathat.ezkey) {
      logger.error('empty obj.stathat.ezkey')
      return null
    }
    if (!['string', 'undefined'].includes(typeof obj.stathat.prefix)) {
      logger.error(
        'unexpect obj.stathat.prefix type',
        typeof obj.stathat.prefix
      )
      return null
    }
  }

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
