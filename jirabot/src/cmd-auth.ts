import * as Message from './message'
import {Context} from './context'
import * as Errors from './errors'
import * as JiraOauth from './jira-oauth'
import * as Jira from './jira'
import * as Utils from './utils'

const replyInPrivate = async (
  context: Context,
  parsedMessage: Message.AuthMessage,
  body: string
): Promise<any> => {
  const channel = {
    name: `${parsedMessage.context.senderUsername},${
      context.bot.myInfo().username
    }`,
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
  parsedMessage: Message.AuthMessage,
  body: string
): Promise<any> => {
  try {
    return await Utils.replyToMessageContext(
      context,
      parsedMessage.context,
      body
    )
  } catch {
    return
  }
}

export default async (
  context: Context,
  parsedMessage: Message.AuthMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  const teamJiraConfigRet = await context.configs.getTeamJiraConfig(
    parsedMessage.context.teamName
  )
  if (teamJiraConfigRet.type === Errors.ReturnType.Error) {
    switch (teamJiraConfigRet.error.type) {
      case Errors.ErrorType.Unknown:
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          teamJiraConfigRet.error
        )
        return Errors.makeError(undefined)
      case Errors.ErrorType.KVStoreNotFound:
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
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
      parsedMessage,
      `Please allow Jirabot to access your Jira account here: ${url}`
    )
    replyInTeamConvo(
      context,
      parsedMessage,
      'A link has been sent to you in private message. Please continue from there to allow Jirabot to access your account'
    )
  }

  const oauthRet = await JiraOauth.doOauth(context, teamJiraConfig, onAuthUrl)
  if (oauthRet.type === Errors.ReturnType.Error) {
    switch (oauthRet.error.type) {
      case Errors.ErrorType.Unknown:
      case Errors.ErrorType.Timeout:
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          oauthRet.error
        )
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
      parsedMessage.context,
      jiraAccountIDRet.error
    )
    return Errors.makeError(undefined)
  }
  const jiraAccountID = jiraAccountIDRet.result

  const updateRet = await context.configs.updateTeamUserConfig(
    parsedMessage.context.teamName,
    parsedMessage.context.senderUsername,
    undefined,
    {
      jiraAccountID,
      accessToken: oauthResult.accessToken,
      tokenSecret: oauthResult.tokenSecret,
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
  replyInPrivate(
    context,
    parsedMessage,
    `Success! You can now use Jirabot in ${parsedMessage.context.teamName}.`
  )

  return Errors.makeResult(undefined)
}
