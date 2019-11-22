import Bot from './bot'
import {init} from './context'
import * as Config from './config'

const config = Config.parse(process.env.JIRABOT_CONFIG || '')
if (!config) {
  console.error('invalid config')
  console.error(process.env.JIRABOT_CONFIG)
  process.exit(1)
} else {
  init(config).then(Bot)
}
