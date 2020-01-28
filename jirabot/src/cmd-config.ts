import * as Message from './message'
import {Context} from './context'
import * as Configs from './configs'
import * as Errors from './errors'
import * as JiraOauth from './jira-oauth'
import * as Utils from './utils'
import * as Jira from './jira'

const makeNewTeamChannelConfig = async (
  context: Context,
  messageContext: Message.MessageContext,
  oldConfig: Configs.TeamChannelConfig,
  name: string,
  value: string
): Promise<Errors.ResultOrError<
  Configs.TeamChannelConfig,
  | Errors.UnknownParamError
  | Errors.InvalidJiraFieldError
  | Errors.JirabotNotEnabledError
  | Errors.UnknownError
>> => {
  switch (name) {
    case 'defaultNewIssueProject': {
      const jiraMetadataRet = await Jira.getJiraMetadata(
        context,
        messageContext.teamName,
        messageContext.senderUsername
      )
      if (jiraMetadataRet.type === Errors.ReturnType.Error) {
        return jiraMetadataRet
      }
      const jiraMetadata = jiraMetadataRet.result
      const normalizedProject = jiraMetadata.normalizeProject(value)
      if (!normalizedProject) {
        return Errors.makeError({
          type: Errors.ErrorType.InvalidJiraField,
          fieldType: Errors.InvalidJiraFieldType.Project,
          invalidValue: value,
          validValues: jiraMetadata.projects(),
        })
      }
      return Errors.makeResult<Configs.TeamChannelConfig>({
        ...oldConfig,
        defaultNewIssueProject: normalizedProject.toLowerCase(),
      })
    }
    default:
      return Errors.makeError<Errors.UnknownParamError>({
        type: Errors.ErrorType.UnknownParam,
        paramName: name,
      })
  }
}

const replyChat = async (
  context: Context,
  parsedMessage: Message.ConfigMessage,
  body: string
): Promise<any> => {
  try {
    return Utils.replyToMessageContext(context, parsedMessage.context, body)
  } catch {
    return
  }
}

const channelConfigToMessageBody = (
  channelConfig: Configs.TeamChannelConfig,
  opening: string
) =>
  `${opening}

*defaultNewIssueProject:* ${channelConfig.defaultNewIssueProject ||
    '<undefined>'}

When creating a new issue, one can omit the \`in <project>\` part if \`defaultNewIssueProject\` is set.
`

const handleChannelConfig = async (
  context: Context,
  parsedMessage: Message.ConfigMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  if (parsedMessage.configType !== Message.ConfigType.Channel) {
    return Errors.makeError(undefined)
  }
  loop: for (let attempt = 0; attempt < 2; ++attempt) {
    const oldConfigRet = await context.configs.getTeamChannelConfig(
      parsedMessage.context.teamName,
      parsedMessage.context.conversationId
    )
    let oldCachedConfig = undefined
    let newConfigBase = undefined
    if (oldConfigRet.type === Errors.ReturnType.Error) {
      switch (oldConfigRet.error.type) {
        case Errors.ErrorType.Unknown:
          Errors.reportErrorAndReplyChat(
            context,
            parsedMessage.context,
            oldConfigRet.error
          )
          return Errors.makeError(undefined)
        case Errors.ErrorType.KVStoreNotFound:
          newConfigBase = Configs.emptyTeamChannelConfig
          break
        default:
          let _: never = oldConfigRet.error
      }
    } else {
      oldCachedConfig = oldConfigRet.result
      newConfigBase = oldCachedConfig.config
    }

    if (!parsedMessage.toSet) {
      oldCachedConfig
        ? replyChat(
            context,
            parsedMessage,
            channelConfigToMessageBody(
              oldCachedConfig.config,
              'Current config for this channel:'
            )
          )
        : replyChat(
            context,
            parsedMessage,
            channelConfigToMessageBody(
              Configs.emptyTeamChannelConfig,
              'You do not have any configuration specific to this channel yet. Available configurations:'
            )
          )
      return Errors.makeResult(undefined)
    }

    const newConfigRet = await makeNewTeamChannelConfig(
      context,
      parsedMessage.context,
      newConfigBase,
      parsedMessage.toSet.name,
      parsedMessage.toSet.value
    )
    if (newConfigRet.type === Errors.ReturnType.Error) {
      Errors.reportErrorAndReplyChat(
        context,
        parsedMessage.context,
        newConfigRet.error
      )
      return Errors.makeError(undefined)
    }
    const newConfig = newConfigRet.result

    const updateRet = await context.configs.updateTeamChannelConfig(
      parsedMessage.context.teamName,
      parsedMessage.context.conversationId,
      oldCachedConfig,
      newConfig
    )
    if (updateRet.type === Errors.ReturnType.Error) {
      switch (updateRet.error.type) {
        case Errors.ErrorType.KVStoreRevision:
          continue loop
        case Errors.ErrorType.Unknown:
          Errors.reportErrorAndReplyChat(
            context,
            parsedMessage.context,
            updateRet.error
          )
          return Errors.makeError(undefined)
        default:
          let _: never = updateRet.error
      }
    } else {
      replyChat(
        context,
        parsedMessage,
        channelConfigToMessageBody(
          newConfig,
          'Channel configuration successfully updated. Current configuration for this channel:'
        )
      )
      return Errors.makeResult(undefined)
    }
  }
  return Errors.makeError(undefined)
}

const jiraConfigToMessageBody = (
  context: Context,
  jiraConfig: Configs.TeamJiraConfig
) =>
  `This team is now configured for \`${jiraConfig.jiraHost}\`. ` +
  "If you haven't, here are instructions for connecting on Jira side:\n" +
  'Go to the application links section of Jira admin settings: ' +
  `https://${jiraConfig.jiraHost}/plugins/servlet/applinks/listApplicationLinks` +
  ', and create an application link of type "Generic Application".' +
  ` Use \`${context.botConfig.httpAddressPrefix}\` as the URL of the application.` +
  '\n\nAfter the application link has been created, edit the link and configure "Incoming Authentication" as following:' +
  `\n\n*Consumer Key:* \`${jiraConfig.jiraAuth.consumerKey}\`` +
  '\n*Public Key:* \n```\n' +
  jiraConfig.jiraAuth.publicKey +
  '```\n' +
  '\nOther fields can be empty or arbitrary values.' +
  '\n\nAfter this has been done, any user in this team can use `!jira auth` to connect their account with Jirabot. You can also use `jira config channel` to customize Jirabot for each channel in this team.'

const handleTeamConfig = async (
  context: Context,
  parsedMessage: Message.ConfigMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  if (parsedMessage.configType !== Message.ConfigType.Team) {
    return Errors.makeError(undefined)
  }
  const teamJiraConfigRet = await context.configs.getTeamJiraConfig(
    parsedMessage.context.teamName
  )
  let oldCachedConfig = undefined
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
        break
      default:
        let _: never = teamJiraConfigRet.error
    }
  } else {
    oldCachedConfig = teamJiraConfigRet.result
  }
  if (!parsedMessage.toSet) {
    oldCachedConfig
      ? replyChat(
          context,
          parsedMessage,
          jiraConfigToMessageBody(context, oldCachedConfig.config)
        )
      : Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          Errors.JirabotNotEnabledForTeamError
        )
    return Errors.makeResult(undefined)
  }
  switch (parsedMessage.toSet.name) {
    case 'jiraHost':
      // TODO check admin
      const detailsRet = await JiraOauth.generateNewJiraLinkDetails()
      if (detailsRet.type === Errors.ReturnType.Error) {
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          detailsRet.error
        )
        return Errors.makeError(undefined)
      }
      const details = detailsRet.result

      const newConfig = {
        jiraHost: parsedMessage.toSet.value.replace(/\/+$/, ''),
        jiraAuth: {
          consumerKey: details.consumerKey,
          publicKey: details.publicKey,
          privateKey: details.privateKey,
        },
        issueTypes: [] as Array<string>,
        projects: [] as Array<string>,
      }
      const updateRet = await context.configs.updateTeamJiraConfig(
        parsedMessage.context.teamName,
        // just override setting in this case and skip the revision check
        undefined,
        newConfig
      )
      if (updateRet.type === Errors.ReturnType.Error) {
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          updateRet.error
        )
        return Errors.makeError(undefined)
      }
      replyChat(
        context,
        parsedMessage,
        jiraConfigToMessageBody(context, newConfig)
      )
      return Errors.makeResult(undefined)
    default:
      Errors.reportErrorAndReplyChat(context, parsedMessage.context, {
        type: Errors.ErrorType.UnknownParam,
        paramName: name,
      })
      return Errors.makeError(undefined)
  }
}

export default async (
  context: Context,
  parsedMessage: Message.ConfigMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  switch (parsedMessage.configType) {
    case Message.ConfigType.Team:
      return await handleTeamConfig(context, parsedMessage)
    case Message.ConfigType.Channel:
      return await handleChannelConfig(context, parsedMessage)
  }
}
