import https from 'https'
import {BotConfig} from './bot-config'
import logger from './logger'

export default class {
  private ezkey: string
  private prefix: string

  constructor(botConfig: BotConfig) {
    this.ezkey = botConfig.stathat?.ezkey || ''
    this.prefix = botConfig.stathat?.prefix || ''
  }

  private _post(statname: string, isCount: boolean, n: number): Promise<void> {
    if (!this.ezkey) {
      return Promise.resolve()
    }

    const payload = `ezkey=${this.ezkey}&stat=${this.prefix + statname}&${
      isCount ? 'count' : 'value'
    }=${n}`
    return new Promise((resolve, reject) => {
      const req = https
        .request(
          {
            hostname: 'api.stathat.com',
            path: '/ez',
            method: 'POST',
            headers: {
              'Content-Type': 'application/x-www-form-urlencoded',
              'Content-Length': payload.length,
            },
          },
          res =>
            res.statusCode >= 200 || res.statusCode < 300
              ? resolve()
              : reject(new Error(`unexpected statusCode ${res.statusCode}`))
        )
        .on('error', err => reject(err))
      req.end(payload)
    })
  }

  private async post(
    statname: string,
    isCount: boolean,
    n: number
  ): Promise<void> {
    return this._post(statname, isCount, n).catch(err =>
      logger.warn({msg: 'stathat error', err})
    )
  }

  postCount(name: string, count: number): Promise<void> {
    return this.post(name, true, count)
  }

  postValue(name: string, value: number): Promise<void> {
    return this.post(name, false, value)
  }
}
