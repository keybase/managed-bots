import Bot from './bot'
import {init} from './context'
import * as BotConfig from './bot-config'

const botConfig = BotConfig.parse(process.env.JIRABOT_CONFIG || '')
if (!botConfig) {
  console.error('invalid bot-config')
  console.error(process.env.JIRABOT_CONFIG)
  process.exit(1)
} else {
  init(botConfig).then(Bot)
}
