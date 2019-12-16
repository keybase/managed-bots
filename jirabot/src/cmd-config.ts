import * as Message from './message'
import {Context} from './context'
import * as Configs from './configs'
import * as Errors from './errors'
import * as JiraOauth from './jira-oauth'

const makeNewTeamChannelConfig = (
  oldConfig: Configs.TeamChannelConfig,
  name: string,
  value: string
): Errors.ResultOrError<
  Configs.TeamChannelConfig,
  Errors.UnknownParamError | Errors.DisabledProjectError
> => {
  switch (name) {
    case 'enabledProjects': {
      // TODO check for project existence with jira
      return Errors.makeResult<Configs.TeamChannelConfig>({
        ...oldConfig,
        enabledProjects: value
          .split(',')
          .filter(Boolean)
          .map(s => s.toLowerCase()),
      })
    }
    case 'defaultNewIssueProject': {
      if (!oldConfig.enabledProjects.includes(value)) {
        return Errors.makeError<Errors.DisabledProjectError>({
          type: Errors.ErrorType.DisabledProject,
          projectName: value,
        })
      }
      return Errors.makeResult<Configs.TeamChannelConfig>({
        ...oldConfig,
        defaultNewIssueProject: value.toLowerCase(),
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
    return context.bot.chat.send(parsedMessage.context.chatChannel, {
      body,
    })
  } catch {
    return
  }
}

const channelConfigToMessageBody = (channelConfig: Configs.TeamChannelConfig) =>
  `Current config for this channel:

*defaultNewIssueProject:* ${channelConfig.defaultNewIssueProject ||
    '<undefined>'}
*enabledProjects:* ${channelConfig.enabledProjects.join(',') || '<empty>'}

In this channel you can only interract with projects in \`enabledProjects\`. When creating a new issue, one can omit the \`in <project>\` part if \`defaultNewIssueProject\` is set.
`

const handleChannelConfig = async (
  context: Context,
  parsedMessage: Message.ConfigMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  if (parsedMessage.configType !== Message.ConfigType.Channel) {
    return Errors.makeError(undefined)
  }
  loop: for (let attempt = 0; attempt < 2; ++attempt) {
    const oldConfigResultOrError = await context.configs.getTeamChannelConfig(
      parsedMessage.context.teamName,
      parsedMessage.context.channelName
    )
    let oldCachedConfig = undefined
    let newConfigBase = undefined
    if (oldConfigResultOrError.type === Errors.ReturnType.Error) {
      switch (oldConfigResultOrError.error.type) {
        case Errors.ErrorType.Unknown:
          Errors.reportErrorAndReplyChat(
            context,
            parsedMessage.context,
            oldConfigResultOrError.error
          )
          return Errors.makeError(undefined)
        case Errors.ErrorType.KVStoreNotFound:
          newConfigBase = Configs.emptyTeamChannelConfig
          break
        default:
          let _: never = oldConfigResultOrError.error
      }
    } else {
      oldCachedConfig = oldConfigResultOrError.result
      newConfigBase = oldCachedConfig.config
    }

    if (!parsedMessage.toSet) {
      oldCachedConfig
        ? replyChat(
            context,
            parsedMessage,
            channelConfigToMessageBody(oldCachedConfig.config)
          )
        : Errors.reportErrorAndReplyChat(
            context,
            parsedMessage.context,
            Errors.JirabotNotEnabledForChannelError
          )
      return Errors.makeResult(undefined)
    }

    const newConfigResultOrError = makeNewTeamChannelConfig(
      newConfigBase,
      parsedMessage.toSet.name,
      parsedMessage.toSet.value
    )
    if (newConfigResultOrError.type === Errors.ReturnType.Error) {
      Errors.reportErrorAndReplyChat(
        context,
        parsedMessage.context,
        newConfigResultOrError.error
      )
      return Errors.makeError(undefined)
    }

    const updateResultOrError = await context.configs.updateTeamChannelConfig(
      parsedMessage.context.teamName,
      parsedMessage.context.channelName,
      oldCachedConfig,
      newConfigResultOrError.result
    )
    if (updateResultOrError.type === Errors.ReturnType.Error) {
      switch (updateResultOrError.error.type) {
        case Errors.ErrorType.KVStoreRevision:
          continue loop
        case Errors.ErrorType.Unknown:
          Errors.reportErrorAndReplyChat(
            context,
            parsedMessage.context,
            updateResultOrError.error
          )
          return Errors.makeError(undefined)
        default:
          let _: never = updateResultOrError.error
      }
    } else {
      return Errors.makeResult(undefined)
    }
  }
  return Errors.makeError(undefined)
}

const jiraConfigToMessageBody = (jiraConfig: Configs.TeamJiraConfig) =>
  `This team is now configured for \`${jiraConfig.jiraHost}\`. ` +
  'In Jira admin settings, create an application link of type "Generic Application".' +
  `\n\n_Tip: Can't find application link settings on Jira? Try the "Search Jira Admin" box in the top right corner of admin settings._` +
  '\n\nAfter the application link has been created, edit the link and configure "Incoming Authentication" as following:' +
  `\n\n*Consumer Key:* \`${jiraConfig.jiraAuth.consumerKey}\`` +
  '\n*Public Key:* \n```\n' +
  jiraConfig.jiraAuth.publicKey +
  '```\n' +
  '\nOther fields can be empty or arbitrary values.'

const handleTeamConfig = async (
  context: Context,
  parsedMessage: Message.ConfigMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  if (parsedMessage.configType !== Message.ConfigType.Team) {
    return Errors.makeError(undefined)
  }
  const teamJiraConfigResultOrError = await context.configs.getTeamJiraConfig(
    parsedMessage.context.teamName
  )
  let oldCachedConfig = undefined
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
        break
      default:
        let _: never = teamJiraConfigResultOrError.error
    }
  } else {
    oldCachedConfig = teamJiraConfigResultOrError.result
  }
  if (!parsedMessage.toSet) {
    oldCachedConfig
      ? replyChat(
          context,
          parsedMessage,
          jiraConfigToMessageBody(oldCachedConfig.config)
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
      const detailsResultOrError = await JiraOauth.generarteNewJiraLinkDetails()
      if (detailsResultOrError.type === Errors.ReturnType.Error) {
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          detailsResultOrError.error
        )
        return Errors.makeError(undefined)
      }
      const details = detailsResultOrError.result
      const newConfig = {
        jiraHost: parsedMessage.toSet.value,
        jiraAuth: {
          consumerKey: details.consumerKey,
          publicKey: details.publicKey,
          privateKey: details.privateKey,
        },
      }
      const updateResultOrError = await context.configs.updateTeamJiraConfig(
        parsedMessage.context.teamName,
        // just override setting in this case and skip the revision check
        undefined,
        newConfig
      )
      if (updateResultOrError.type === Errors.ReturnType.Error) {
        Errors.reportErrorAndReplyChat(
          context,
          parsedMessage.context,
          updateResultOrError.error
        )
        return Errors.makeError(undefined)
      }
      replyChat(context, parsedMessage, jiraConfigToMessageBody(newConfig))
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
