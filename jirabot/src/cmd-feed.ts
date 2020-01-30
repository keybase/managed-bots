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

  let webhookURI: string
  try {
    webhookURI = await jira.subscribe(
      jql,
      [
        Jira.JiraSubscriptionEvents.IssueCreated,
        Jira.JiraSubscriptionEvents.IssueUpdated,
      ],
      `${context.botConfig.httpAddressPrefix}${Constants.jiraWebhookPathname}?urlToken=${urlToken}`
    )
  } catch (err) {
    reportJiraError(context, parsedMessage.context, err)
    return Errors.makeError(undefined)
  }

  let id = 0
  const updateRet = await updateTeamJiraSubscriptions(
    context,
    parsedMessage.context.teamName,
    (oldSubscriptions?: Configs.TeamJiraSubscriptions) => {
      const oldEntries = [...(oldSubscriptions?.entries() || [])]
      id =
        oldEntries.reduce(
          (max: number, [current]) => (max = current > max ? current : max),
          0
        ) + 1
      return new Map([
        ...oldEntries,
        [
          id,
          {
            conversationId: parsedMessage.context.conversationId,
            webhookURI,
            urlToken,
            jql,
          },
        ],
      ])
    }
  )
  if (updateRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      updateRet.error
    )
    return Errors.makeError(undefined)
  }

  const setIndexRet = await context.configs.setOrDeleteJiraSubscriptionIndex(
    urlToken,
    {
      teamname: parsedMessage.context.teamName,
      id,
    }
  )
  if (setIndexRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      setIndexRet.error
    )
    return Errors.makeError(undefined)
  }

  Utils.replyToMessageContext(
    context,
    parsedMessage.context,
    `Subscribed to ${parsedMessage.project}:\n${id}: \`${jql}\``
  )
  return Errors.makeResult(undefined)
}

const unsubscribe = async (
  context: Context,
  parsedMessage: Message.FeedUnsubscribeMessage,
  jira: Jira.JiraClientWrapper
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  if (!parsedMessage.subscriptionID) {
    // TODO we don't support subscribing from all yet
    return Errors.makeError(undefined)
  }

  const getSubRet = await context.configs.getTeamJiraSubscriptions(
    parsedMessage.context.teamName
  )
  if (getSubRet.type === Errors.ReturnType.Error) {
    getSubRet.error.type === Errors.ErrorType.KVStoreNotFound
      ? Utils.replyToMessageContext(
          context,
          parsedMessage.context,
          `There is no active subscription in this team.`
        )
      : Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          getSubRet.error
        )
    return Errors.makeError(undefined)
  }
  const subscription = getSubRet.result.config.get(parsedMessage.subscriptionID)
  if (!subscription) {
    Utils.replyToMessageContext(
      context,
      parsedMessage.context,
      `Unknown subscription ID ${parsedMessage.subscriptionID}`
    )
    return Errors.makeError(undefined)
  }

  try {
    await jira.unsubscribe(subscription.webhookURI)
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
              ([subscriptionID]) =>
                subscriptionID !== parsedMessage.subscriptionID
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

  const deleteIndexRet = await context.configs.setOrDeleteJiraSubscriptionIndex(
    subscription.urlToken
  )
  if (deleteIndexRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      deleteIndexRet.error
    )
    return Errors.makeError(undefined)
  }

  Utils.replyToMessageContext(
    context,
    parsedMessage.context,
    `Unsubscribed ${parsedMessage.subscriptionID}.`
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
  const subscriptions =
    getSubRet.type === Errors.ReturnType.Ok
      ? [...getSubRet.result.config?.entries()]
      : []
  if (parsedMessage.allChannelsInTeam) {
    Utils.replyToMessageContext(
      context,
      parsedMessage.context,
      `You have ${subscriptions.length || 'no'} active subscription${
        subscriptions.length > 1 ? 's' : ''
      } in *this team*. Note that some of them might not be for this channel.` +
        subscriptions.reduce(
          (str, [subscriptionID, sub]) =>
            str + `\n${subscriptionID}: ${sub.jql}`,
          ''
        )
    )
  } else {
    const channelSubscriptions = subscriptions.filter(
      ([_, {conversationId}]) =>
        conversationId === parsedMessage.context.conversationId
    )
    const otherChannelSubscriptions =
      subscriptions.length - channelSubscriptions.length
    Utils.replyToMessageContext(
      context,
      parsedMessage.context,
      `You have ${channelSubscriptions.length || 'no'} active subscription${
        channelSubscriptions.length > 1 ? 's' : ''
      } in *this channel*.${
        otherChannelSubscriptions > 0
          ? ` There ${
              otherChannelSubscriptions > 1 ? 'are' : 'is'
            } ${otherChannelSubscriptions} subscription${
              otherChannelSubscriptions > 1 ? 's' : ''
            } in other channels of this team. Use \`!jira feed list all\` to view all.`
          : ''
      }` +
        channelSubscriptions.reduce(
          (str, [subscriptionID, sub]) =>
            str + `\n${subscriptionID}: ${sub.jql}`,
          ''
        )
    )
  }
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
