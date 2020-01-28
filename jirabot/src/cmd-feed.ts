import * as Message from './message'
import {Context} from './context'
import * as Errors from './errors'
import * as Jira from './jira'
import * as Configs from './configs'
import * as Constants from './constants'
import * as Utils from './utils'

const updateTeamJiraSubscriptions = async (
  context: Context,
  teamname: string,
  updater: (
    oldSubscriptions?: Configs.TeamJiraSubscriptions
  ) => Configs.TeamJiraSubscriptions
): Promise<Errors.ResultOrError<undefined, Errors.UnknownError>> => {
  loop: for (let attempt = 0; attempt < 2; ++attempt) {
    const getSubRet = await context.configs.getTeamJiraSubscriptions(teamname)
    let oldSubscriptions = undefined
    if (getSubRet.type === Errors.ReturnType.Error) {
      switch (getSubRet.error.type) {
        case Errors.ErrorType.Unknown:
          return Errors.makeError(getSubRet.error)
        case Errors.ErrorType.KVStoreNotFound:
          break
        default:
          let _: never = getSubRet.error
      }
    } else {
      oldSubscriptions = getSubRet.result
    }
    const newSubscriptions = updater(oldSubscriptions?.config)
    const updateRet = await context.configs.updateTeamJiraSubscriptions(
      teamname,
      oldSubscriptions,
      newSubscriptions
    )
    if (updateRet.type === Errors.ReturnType.Error) {
      switch (updateRet.error.type) {
        case Errors.ErrorType.Unknown:
          return Errors.makeError(updateRet.error)
        case Errors.ErrorType.KVStoreRevision:
          continue loop
        default:
          let _: never = updateRet.error
      }
    }
    return Errors.makeResult(undefined)
  }
  return Errors.makeUnknownError('update kvstore failed')
}

const reportJiraError = (
  context: Context,
  messageContext: Message.MessageContext,
  err: any
) => {
  let statusCode = undefined
  if (typeof err.statusCode === 'number') {
    statusCode = err.statusCode
  } else {
    try {
      const obj = JSON.parse(err)
      if (typeof obj.statusCode === 'number') {
        statusCode = obj.statusCode
      }
    } catch {}
  }
  Errors.reportErrorAndReplyChat(
    context,
    messageContext,
    statusCode === 403
      ? {type: Errors.ErrorType.JiraNoPermission}
      : Errors.makeUnknownError(err).error
  )
}

const subscribe = async (
  context: Context,
  parsedMessage: Message.FeedSubscribeMessage,
  jira: Jira.JiraClientWrapper
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  const urlToken = await Utils.randomString('jira-subscription')
  const jql = Jira.projectToJqlFilter(parsedMessage.project)

  let webhookID: number
  try {
    webhookID = await jira.subscribe(
      jql,
      [
        Jira.JiraSubscriptionEvents.IssueCreated,
        Jira.JiraSubscriptionEvents.IssueUpdated,
      ],
      `${context.botConfig.httpAddressPrefix}${Constants.jiraWebhookPathname}?team=${parsedMessage.context.teamName}&urlToken=${urlToken}`
    )
  } catch (err) {
    reportJiraError(context, parsedMessage.context, err)
    return Errors.makeError(undefined)
  }

  const updateRet = await updateTeamJiraSubscriptions(
    context,
    parsedMessage.context.teamName,
    (oldSubscriptions: Configs.TeamJiraSubscriptions) =>
      new Map([
        ...(oldSubscriptions?.entries() || []),
        [
          webhookID,
          {
            conversationId: parsedMessage.context.conversationId,
            urlToken,
            jql,
          },
        ],
      ])
  )
  if (updateRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      updateRet.error
    )
    return Errors.makeError(undefined)
  }
  Utils.replyToMessageContext(
    context,
    parsedMessage.context,
    `Subscribed to ${parsedMessage.project}:\n${webhookID}: ${jql}`
  )
  return Errors.makeResult(undefined)
}

const unsubscribe = async (
  context: Context,
  parsedMessage: Message.FeedUnsubscribeMessage,
  jira: Jira.JiraClientWrapper
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  if (!parsedMessage.webhookID) {
    return Errors.makeError(undefined)
  }
  try {
    await jira.unsubscribe(parsedMessage.webhookID)
  } catch (err) {
    reportJiraError(context, parsedMessage.context, err)
    return Errors.makeError(undefined)
  }
  const updateRet = await updateTeamJiraSubscriptions(
    context,
    parsedMessage.context.teamName,
    (oldSubscriptions: Configs.TeamJiraSubscriptions) =>
      oldSubscriptions
        ? new Map(
            [...oldSubscriptions.entries()].filter(
              ([webhookID]) => webhookID !== parsedMessage.webhookID
            )
          )
        : (new Map() as Configs.TeamJiraSubscriptions)
  )
  if (updateRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      updateRet.error
    )
    return Errors.makeError(undefined)
  }
  Utils.replyToMessageContext(
    context,
    parsedMessage.context,
    `Unsubscribed ${parsedMessage.webhookID}.`
  )
  return Errors.makeResult(undefined)
}

const list = async (
  context: Context,
  parsedMessage: Message.FeedListMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  const getSubRet = await context.configs.getTeamJiraSubscriptions(
    parsedMessage.context.teamName
  )
  if (
    getSubRet.type === Errors.ReturnType.Error &&
    getSubRet.error.type !== Errors.ErrorType.KVStoreNotFound
  ) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      getSubRet.error
    )
    return Errors.makeError(undefined)
  }
  const oldSubscriptions =
    getSubRet.type === Errors.ReturnType.Ok ? getSubRet.result : undefined
  Utils.replyToMessageContext(
    context,
    parsedMessage.context,
    `You have ${oldSubscriptions?.config.size ||
      'no'} active subscriptions. Use !jira feed subscribe|unsubscribe to make changes.` +
      [...(oldSubscriptions?.config?.entries() || [])].reduce(
        (str, [webhookID, sub]) => str + `\n${webhookID}: ${sub.jql}`,
        ''
      )
  )
  return Errors.makeResult(undefined)
}

export default async (
  context: Context,
  parsedMessage: Message.FeedMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  const jiraRet = await context.getJiraFromTeamnameAndUsername(
    context,
    parsedMessage.context.teamName,
    parsedMessage.context.senderUsername
  )
  if (jiraRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      jiraRet.error
    )
    return Errors.makeError(undefined)
  }
  const jira = jiraRet.result

  switch (parsedMessage.feedMessageType) {
    case Message.FeedMessageType.Subscribe:
      return subscribe(context, parsedMessage, jira)
    case Message.FeedMessageType.Unsubscribe:
      return unsubscribe(context, parsedMessage, jira)
    case Message.FeedMessageType.List:
      return list(context, parsedMessage)
  }
}
