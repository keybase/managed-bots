import logger from './logger'

export type BotConfig = {
  httpAddressPrefix: string // e.g. https://example.com
  keybase: {
    username: string
    paperkey: string
  }
  allowedTeams: Array<string>
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
