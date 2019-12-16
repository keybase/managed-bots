import * as Message from './message'
import {Context} from './context'
import * as Configs from './configs'
import * as Errors from './errors'
import * as JiraOauth from './jira-oauth'
import * as Jira from './jira'

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
    return context.bot.chat.send(parsedMessage.context.chatChannel, {
      body,
    })
  } catch {
    return
  }
}

export default async (
  context: Context,
  parsedMessage: Message.AuthMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  const teamJiraConfigResultOrError = await context.configs.getTeamJiraConfig(
    parsedMessage.context.teamName
  )
  if (teamJiraConfigResultOrError.type === Errors.ReturnType.Error) {
    switch (teamJiraConfigResultOrError.error.type) {
      case Errors.ErrorType.Unknown:
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          teamJiraConfigResultOrError.error
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
        let _: never = teamJiraConfigResultOrError.error
        return Errors.makeError(undefined)
    }
  }
  const teamJiraConfig = teamJiraConfigResultOrError.result.config

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

  const oauthResultOrError = await JiraOauth.doOauth(
    context,
    teamJiraConfig,
    onAuthUrl
  )
  if (oauthResultOrError.type === Errors.ReturnType.Error) {
    switch (oauthResultOrError.error.type) {
      case Errors.ErrorType.Unknown:
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          oauthResultOrError.error
        )
        return Errors.makeError(undefined)
      case Errors.ErrorType.Timeout:
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          oauthResultOrError.error
        )
        return Errors.makeError(undefined)
      default:
        let _: never = oauthResultOrError.error
        return Errors.makeError(undefined)
    }
  }
  const oauthResult = oauthResultOrError.result

  const jiraAccountIDResultOrError = await Jira.getAccountId(
    teamJiraConfig,
    oauthResult.accessToken,
    oauthResult.tokenSecret
  )
  if (jiraAccountIDResultOrError.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      jiraAccountIDResultOrError.error
    )
    return Errors.makeError(undefined)
  }
  const jiraAccountID = jiraAccountIDResultOrError.result

  const updateResultOrError = await context.configs.updateTeamUserConfig(
    parsedMessage.context.teamName,
    parsedMessage.context.senderUsername,
    undefined,
    {
      jiraAccountID,
      accessToken: oauthResult.accessToken,
      tokenSecret: oauthResult.tokenSecret,
    }
  )
  if (updateResultOrError.type === Errors.ReturnType.Error) {
    Errors.reportErrorAndReplyChat(
      context,
      parsedMessage.context,
      updateResultOrError.error
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
