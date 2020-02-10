import {Context} from './context'
import logger from './logger'
import * as Errors from './errors'

const postStats = async (context: Context): Promise<void> => {
  const indicesRet = await context.configs.listAllJiraSubscriptionIndices()
  if (indicesRet.type !== Errors.ReturnType.Ok) {
    logger.warn({msg: 'postStats', error: indicesRet.error})
    return
  }
  const indices = indicesRet.result

  const subscriptions = indices.length
  const teams = new Set(indices.map(index => index.teamname)).size

  context.stathat.postValue('subscriptions', subscriptions)
  context.stathat.postValue('teams', teams)
}

const statInterval = 60 * 1000 // 1min

export default (context: Context) => {
  postStats(context)
  setInterval(() => postStats(context), statInterval)
}
