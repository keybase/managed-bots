import ChatTypes from 'keybase-bot/lib/types/chat1'
import * as Utils from './utils'
import {Context} from './context'
import * as Errors from './errors'
import logger from './logger'

export enum BotMessageType {
  Unknown = 'unknown',
  Create = 'create',
  Search = 'search',
  Comment = 'comment',
  Reacji = 'reacji',
  Config = 'config',
  Auth = 'auth',
}

export type MessageContext = Readonly<{
  chatChannel: ChatTypes.ChatChannel

  senderUsername: string
  teamName: string
  channelName: string
}>

type UnknownMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Unknown
  error?: string
}>

export type CreateMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Create
  name: string
  project: string
  assignee: string
  description: string
  issueType: string
}>

export type SearchMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Search
  query: string
  project: string
  status: string
  assignee: string
}>

export type CommentMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Comment
  ticket: string
  comment: string
}>

export type ReacjiMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Reacji
  reactToID: number
  emoji: string
}>

export enum ConfigType {
  Team = 'team',
  Channel = 'channel',
}

export type ConfigMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Config
  configType: ConfigType
  toSet?: Readonly<{
    name: string
    value: string
  }>
}>

export type AuthMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Auth
}>

export type Message =
  | UnknownMessage
  | SearchMessage
  | CommentMessage
  | ReacjiMessage
  | CreateMessage
  | ConfigMessage
  | AuthMessage

const getTextMessage = (message: ChatTypes.MsgSummary): string | undefined => {
  if (!message || !message.content) {
    return undefined
  }
  if (
    message.content.type === 'text' &&
    message.content.text &&
    typeof message.content.text.body === 'string'
  ) {
    return message.content.text.body
  }
  if (
    message.content.type === 'edit' &&
    message.content.edit &&
    typeof message.content.edit.body === 'string'
  ) {
    return message.content.edit.body
  }
  return undefined
}

const checkAndGetProjectName = async (
  context: Context,
  messageContext: MessageContext,
  project: string,
  required: boolean
): Promise<Errors.ResultOrError<
  string,
  | Errors.UnknownError
  | Errors.DisabledProjectError
  | Errors.ChannelNotConfiguredError
  | Errors.MissingProjectError
>> => {
  const teamChannelConfigResultOrError = await context.configs.getTeamChannelConfig(
    messageContext.teamName,
    messageContext.channelName
  )
  if (teamChannelConfigResultOrError.type === Errors.ReturnType.Error) {
    switch (teamChannelConfigResultOrError.error.type) {
      case Errors.ErrorType.Unknown:
        return Errors.makeError(teamChannelConfigResultOrError.error)
      case Errors.ErrorType.KVStoreNotFound:
        return Errors.channelNotConfiguredError
    }
  }
  const teamChannelConfig = teamChannelConfigResultOrError.result
  if (!project) {
    const defaultProject = teamChannelConfig.config.defaultNewIssueProject
    return required
      ? defaultProject
        ? Errors.makeResult<string>(defaultProject)
        : Errors.missingProjectError
      : Errors.makeResult<string>('')
  }
  if (!teamChannelConfig.config.enabledProjects.includes(project)) {
    return Errors.makeError({
      type: Errors.ErrorType.DisabledProject,
      projectName: project,
    })
  }
  return Errors.makeResult<string>(project.toLowerCase())
}

const checkStatusError = (
  context: Context,
  status: string
): string | undefined =>
  status && !context.botConfig.jira.status.includes(status)
    ? `invalid status: ${status} is not one of ${Utils.humanReadableArray(
        context.botConfig.jira.status
      )}`
    : undefined

const checkAssigneeError = (
  context: Context,
  assignee: string
): string | undefined =>
  assignee && !context.botConfig.jira.usernameMapper[assignee]
    ? `invalid assignee: ${assignee} is not one of ${Utils.humanReadableArray(
        Object.keys(context.botConfig.jira.usernameMapper)
      )}`
    : undefined

const checkIssueTypeError = (
  context: Context,
  issueType: string
): string | undefined =>
  issueType && !context.botConfig.jira.issueTypes.includes(issueType)
    ? `invalid issueType: ${issueType} is not one of ${Utils.humanReadableArray(
        context.botConfig.jira.issueTypes
      )}`
    : undefined

const extractArgsAfterCommand = <K extends string>(
  fields: Array<string>,
  keys: Set<K>
): {
  args: Partial<Record<K, string>>
  rest: Array<string>
} => {
  const args: Partial<Record<K, string>> = {}
  let i = 0
  for (; i + 1 < fields.length; i += 2) {
    if (keys.has(fields[i] as K)) {
      args[fields[i] as K] = fields[i + 1]
    } else {
      break
    }
  }
  return {args, rest: fields.slice(i)}
}

const msgSummaryToMessageContext = (
  kbMessage: ChatTypes.MsgSummary
): MessageContext => ({
  chatChannel: kbMessage.channel,
  senderUsername: kbMessage.sender.username,
  teamName: kbMessage.channel.name,
  channelName: kbMessage.channel.topicName ?? '',
})

const shouldProcessMessageContext = (
  context: Context,
  messageContext: MessageContext
) => {
  if (messageContext.chatChannel.membersType !== 'team') {
    return false
  }
  if (
    context.botConfig.allowedTeams &&
    !context.botConfig.allowedTeams.includes(messageContext.teamName)
  ) {
    return false
  }
  return true
}

const newArgs = new Set(['in', 'for', 'assignee'])
const searchArgs = new Set(['in', 'assignee', 'status'])
const commentArgs = new Set(['on'])

export const parseMessage = async (
  context: Context,
  kbMessage: ChatTypes.MsgSummary
): Promise<Message | undefined> => {
  const messageContext = msgSummaryToMessageContext(kbMessage)
  if (!shouldProcessMessageContext(context, messageContext)) {
    logger.debug({
      msg: 'ignoring message from',
      teamName: messageContext.teamName,
    })
    return undefined
  }

  if (
    kbMessage.channel.membersType !== 'team' ||
    kbMessage.channel.topicType !== 'chat'
  ) {
    return undefined
  }

  const textBody = getTextMessage(kbMessage)
  if (!textBody) {
    return undefined
  }

  if (!textBody.startsWith('!jira')) {
    return undefined
  }

  const fields = Utils.split2(textBody)

  switch (fields[1]) {
    case 'new': {
      const issueTypeFromInputCaseMapped = context.botConfig.jira._issueTypeInsensitiveToIssueType(
        fields[2]
      )
      const issueType = context.botConfig.jira.issueTypes.includes(
        issueTypeFromInputCaseMapped
      )
        ? issueTypeFromInputCaseMapped
        : ''

      const issueTypeError = checkIssueTypeError(context, issueType)
      if (issueTypeError) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: issueTypeError,
        }
      }

      const {args, rest} = extractArgsAfterCommand(
        fields.slice(issueType ? 3 : 2),
        newArgs
      )
      if (rest.length < 1) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: '`!jira new` needs at least a summary for the ticket',
        }
      }

      const checkAndGetProjectNameResultOrError = await checkAndGetProjectName(
        context,
        messageContext,
        args.in,
        true
      )
      if (
        checkAndGetProjectNameResultOrError.type === Errors.ReturnType.Error
      ) {
        Errors.reportErrorAndReplyChat(
          context,
          messageContext,
          checkAndGetProjectNameResultOrError.error
        )
        return undefined
      }

      const assignee = args.assignee
        ? args.assignee.replace(/^@+/, '')
        : args.for && args.for.replace(/^@+/, '')
      const assigneeError = checkAssigneeError(context, assignee)
      if (assigneeError) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: assigneeError,
        }
      }

      return {
        context: messageContext,
        type: BotMessageType.Create,
        name: rest[0],
        project: checkAndGetProjectNameResultOrError.result,
        assignee,
        description: rest.slice(1).join(' '),
        issueType,
      }
    }
    case 'search': {
      const {args, rest} = extractArgsAfterCommand(fields.slice(2), searchArgs)
      if (rest.length < 1) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: '`!jira search` needs at least a query',
        }
      }

      const checkAndGetProjectNameResultOrError = await checkAndGetProjectName(
        context,
        messageContext,
        args.in,
        false
      )
      if (
        checkAndGetProjectNameResultOrError.type === Errors.ReturnType.Error
      ) {
        Errors.reportErrorAndReplyChat(
          context,
          messageContext,
          checkAndGetProjectNameResultOrError.error
        )
        return undefined
      }

      const assignee = args.assignee && args.assignee.replace(/^@+/, '')
      const assigneeError = checkAssigneeError(context, assignee)
      if (assigneeError) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: assigneeError,
        }
      }

      const statusError = checkStatusError(context, args.status)
      if (statusError) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: statusError,
        }
      }

      return {
        context: messageContext,
        type: BotMessageType.Search,
        query: rest.join(' '),
        project: checkAndGetProjectNameResultOrError.result,
        assignee,
        status: args.status,
      }
    }
    case 'comment': {
      const {args, rest} = extractArgsAfterCommand(fields.slice(2), commentArgs)
      if (rest.length < 1) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: '`!jira comment` needs a comment to post',
        }
      }
      return {
        context: messageContext,
        type: BotMessageType.Comment,
        ticket: args.on,
        comment: rest.join(' '),
      }
    }
    case 'auth': {
      if (fields.length > 2) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: 'The `auth` command takes no arguments.',
        }
      }
      return {
        context: messageContext,
        type: BotMessageType.Auth,
      }
    }
    case 'config': {
      if (fields[2] !== 'team' && fields[2] !== 'channel') {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: `unknown config target ${fields[2]}`,
        }
      }
      const configType =
        fields[2] === 'team' ? ConfigType.Team : ConfigType.Channel
      const toSetName = fields[3]
      const toSetValue = fields[4]
      if (toSetName && !toSetValue) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: 'setting config parameters requires a value',
        }
      }
      switch (configType) {
        case ConfigType.Team:
          if (toSetName) {
            if (toSetName !== 'jiraHost') {
              return {
                context: messageContext,
                type: BotMessageType.Unknown,
                error: `unknown team config parameter ${toSetName}`,
              }
            }
            return {
              context: messageContext,
              type: BotMessageType.Config,
              configType,
              toSet: {
                name: toSetName,
                value: toSetValue,
              },
            }
          }
          return {
            context: messageContext,
            type: BotMessageType.Config,
            configType,
          }
        case ConfigType.Channel:
          if (toSetName) {
            if (
              !['enabledProjects', 'defaultNewIssueProject'].includes(toSetName)
            ) {
              return {
                context: messageContext,
                type: BotMessageType.Unknown,
                error: `unknown config parameter ${toSetName}`,
              }
            }
            return {
              context: messageContext,
              type: BotMessageType.Config,
              configType,
              toSet: {
                name: toSetName,
                value: toSetValue,
              },
            }
          }
          return {
            context: messageContext,
            type: BotMessageType.Config,
            configType,
          }
      }
    }
    default: {
      return {
        context: messageContext,
        type: BotMessageType.Unknown,
        error: `unknown command ${fields[1]}`,
      }
    }
  }
}
