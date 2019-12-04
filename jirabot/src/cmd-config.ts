import * as Message from './message'
import {Context} from './context'
import * as Configs from './configs'
import * as Errors from './errors'

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

const channelConfigToMessageBody = (
  channelConfig?: Configs.TeamChannelConfig
) =>
  channelConfig
    ? `Current config for this channel:

*defaultNewIssueProject:* ${channelConfig.defaultNewIssueProject ||
        '<undefined>'}
*enabledProjects:* ${channelConfig.enabledProjects.join(',') || '<empty>'}

In this channel you can only interract with projects in \`enabledProjects\`. When creating a new issue, one can omit the \`in <project>\` part if \`defaultNewIssueProject\` is set.
`
    : `This channel has not been configured for Jirabot. In order to use Jirabot, you need to set at least \`enabledProjects\`.`

export default async (
  context: Context,
  parsedMessage: Message.ConfigMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  switch (parsedMessage.configType) {
    case Message.ConfigType.Team:
      replyChat(context, parsedMessage, `team configs are not implemented yet`)
      return Errors.makeError(undefined)
    case Message.ConfigType.Channel:
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
          }
        } else {
          oldCachedConfig = oldConfigResultOrError.result
          newConfigBase = oldCachedConfig.config
        }

        if (!parsedMessage.toSet) {
          replyChat(
            context,
            parsedMessage,
            channelConfigToMessageBody(oldCachedConfig?.config)
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
          }
        } else {
          return Errors.makeResult(undefined)
        }
      }
      return Errors.makeError(undefined)
  }
}
