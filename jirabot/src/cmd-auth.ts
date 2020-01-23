import * as Message from './message'
import {Context} from './context'
import * as Errors from './errors'
import * as JiraOauth from './jira-oauth'
import * as Jira from './jira'
import * as Utils from './utils'

const replyInPrivate = async (
  context: Context,
  messageContext: Message.MessageContext,
  body: string
): Promise<any> => {
  const channel = {
    name: `${messageContext.senderUsername},${context.bot.myInfo().username}`,
    public: false,
    topicType: 'chat',
  }
  try {
    return context.bot.chat.send(channel, {
      body,
    })
  } catch {
    return
  }
}

const replyInTeamConvo = async (
  context: Context,
  messageContext: Message.MessageContext,
  body: string
): Promise<any> => {
  try {
    return await Utils.replyToMessageContext(context, messageContext, body)
  } catch {
    return
  }
}

export const startAuth = async (
  context: Context,
  messageContext: Message.MessageContext
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  const teamJiraConfigRet = await context.configs.getTeamJiraConfig(
    messageContext.teamName
  )
  if (teamJiraConfigRet.type === Errors.ReturnType.Error) {
    switch (teamJiraConfigRet.error.type) {
      case Errors.ErrorType.Unknown:
        Errors.reportErrorAndReplyChat(
          context,
          messageContext,
          teamJiraConfigRet.error
        )
        return Errors.makeError(undefined)
      case Errors.ErrorType.KVStoreNotFound:
        Errors.reportErrorAndReplyChat(
          context,
          messageContext,
          Errors.JirabotNotEnabledForTeamError
        )
        return Errors.makeError(undefined)
      default:
        let _: never = teamJiraConfigRet.error
        return Errors.makeError(undefined)
    }
  }
  const teamJiraConfig = teamJiraConfigRet.result.config

  const onAuthUrl = (url: string) => {
    replyInPrivate(
      context,
      messageContext,
      `Please allow Jirabot to access your Jira account here: ${url}`
    )
    replyInTeamConvo(
      context,
      messageContext,
      'A link has been sent to you in private message. Please continue from there to allow Jirabot to access your account'
    )
  }

  const oauthRet = await JiraOauth.doOauth(context, teamJiraConfig, onAuthUrl)
  if (oauthRet.type === Errors.ReturnType.Error) {
    switch (oauthRet.error.type) {
      case Errors.ErrorType.Unknown:
      case Errors.ErrorType.Timeout:
        Errors.reportErrorAndReplyChat(context, messageContext, oauthRet.error)
        return Errors.makeError(undefined)
      default:
        let _: never = oauthRet.error
        return Errors.makeError(undefined)
    }
  }
  const oauthResult = oauthRet.result

  const jiraAccountIDRet = await Jira.getAccountId(
    teamJiraConfig,
    oauthResult.accessToken,
    oauthResult.tokenSecret
  )
  if (jiraAccountIDRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      messageContext,
      jiraAccountIDRet.error
    )
    return Errors.makeError(undefined)
  }
  const jiraAccountID = jiraAccountIDRet.result

  const updateRet = await context.configs.updateTeamUserConfig(
    messageContext.teamName,
    messageContext.senderUsername,
    undefined,
    {
      jiraAccountID,
      accessToken: oauthResult.accessToken,
      tokenSecret: oauthResult.tokenSecret,
    }
  )
  if (updateRet.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(context, messageContext, updateRet.error)
    return Errors.makeError(undefined)
  }
  replyInPrivate(
    context,
    messageContext,
    `Success! You can now use Jirabot in ${messageContext.teamName}.`
  )

  return Errors.makeResult(undefined)
}

export default async (
  context: Context,
  parsedMessage: Message.AuthMessage
): Promise<Errors.ResultOrError<undefined, undefined>> =>
  startAuth(context, parsedMessage.context)
