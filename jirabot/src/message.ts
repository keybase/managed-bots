import ChatTypes from 'keybase-bot/lib/types/chat1'
import * as Utils from './utils'
import {Context} from './context'
import * as Errors from './errors'
import logger from './logger'
import * as Configs from './configs'
import * as Jira from './jira'

export enum BotMessageType {
  Unknown = 'unknown',
  Create = 'create',
  Search = 'search',
  Comment = 'comment',
  Reacji = 'reacji',
  Config = 'config',
  Auth = 'auth',
  Feed = 'feed',
  Debug = 'debug',
}

export type MessageContext = Readonly<{
  messageID: ChatTypes.MessageID
  chatChannel: ChatTypes.ChatChannel
  senderUsername: string
  teamName: string
  channelName: string
  conversationId: ChatTypes.ConvIDStr
}>

export type UnknownMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Unknown
  error?: string | Errors.Errors
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

export enum FeedMessageType {
  Subscribe = 'subscribe',
  Unsubscribe = 'unsubscribe',
  List = 'list',
}

export type FeedSubscribeMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Feed
  feedMessageType: FeedMessageType.Subscribe
  project: string
  withUpdates: boolean
}>

export type FeedUnsubscribeMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Feed
  feedMessageType: FeedMessageType.Unsubscribe
  subscriptionID?: number // subscirbe all if undefined
}>

export type FeedListMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Feed
  feedMessageType: FeedMessageType.List
  allChannelsInTeam: boolean
}>

export type FeedMessage =
  | FeedSubscribeMessage
  | FeedUnsubscribeMessage
  | FeedListMessage

export enum DebugType {
  LogSend = 'logSend',
  Pprof = 'pprof',
}
export type DebugLogSendMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Debug
  debugType: DebugType.LogSend
}>
export type DebugPprofMessage = Readonly<{
  context: MessageContext
  type: BotMessageType.Debug
  debugType: DebugType.Pprof
  pprofType: 'trace' | 'cpu' | 'heap'
  duration?: number
}>
export type DebugMessage = DebugLogSendMessage | DebugPprofMessage

export type Message =
  | UnknownMessage
  | SearchMessage
  | CommentMessage
  | ReacjiMessage
  | CreateMessage
  | ConfigMessage
  | AuthMessage
  | FeedMessage
  | DebugMessage

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

const getProject = async (
  context: Context,
  messageContext: MessageContext,
  project: string,
  required: boolean
): Promise<Errors.ResultOrError<
  string,
  | Errors.UnknownError
  | Errors.InvalidJiraFieldError
  | Errors.MissingProjectError
  | Errors.JirabotNotEnabledError
  | Errors.UnknownError
>> => {
  const teamChannelConfigRet = await context.configs.getTeamChannelConfig(
    messageContext.teamName,
    messageContext.conversationId
  )
  let teamChannelConfig: Configs.TeamChannelConfig
  if (teamChannelConfigRet.type === Errors.ReturnType.Error) {
    switch (teamChannelConfigRet.error.type) {
      case Errors.ErrorType.Unknown:
        return Errors.makeError(teamChannelConfigRet.error)
      case Errors.ErrorType.KVStoreNotFound:
        teamChannelConfig = Configs.emptyTeamChannelConfig
    }
  } else {
    teamChannelConfig = teamChannelConfigRet.result.config
  }
  if (!project) {
    const defaultProject = teamChannelConfig.defaultNewIssueProject
    return required
      ? defaultProject
        ? Errors.makeResult<string>(defaultProject)
        : Errors.missingProjectError
      : Errors.makeResult<string>('')
  }
  const jiraMetadataRet = await Jira.getJiraMetadata(
    context,
    messageContext.teamName,
    messageContext.senderUsername
  )
  if (jiraMetadataRet.type === Errors.ReturnType.Error) {
    return jiraMetadataRet
  }
  const jiraMetadata = jiraMetadataRet.result
  const normalizedProject = jiraMetadata.normalizeProject(project)
  if (!normalizedProject) {
    return Errors.makeError({
      type: Errors.ErrorType.InvalidJiraField,
      fieldType: Errors.InvalidJiraFieldType.Project,
      invalidValue: project,
      validValues: jiraMetadata.projects(),
    })
  }
  return Errors.makeResult<string>(normalizedProject)
}

const getStatus = async (
  context: Context,
  messageContext: MessageContext,
  status: string
): Promise<Errors.ResultOrError<
  string | undefined,
  | Errors.InvalidJiraFieldError
  | Errors.JirabotNotEnabledError
  | Errors.UnknownError
>> => {
  if (!status) {
    return Errors.makeResult(undefined)
  }
  const jiraMetadataRet = await Jira.getJiraMetadata(
    context,
    messageContext.teamName,
    messageContext.senderUsername
  )
  if (jiraMetadataRet.type === Errors.ReturnType.Error) {
    return jiraMetadataRet
  }
  const jiraMetadata = jiraMetadataRet.result
  const normalizedStatus = jiraMetadata.normalizeStatus(status)
  if (!normalizedStatus) {
    return Errors.makeError({
      type: Errors.ErrorType.InvalidJiraField,
      fieldType: Errors.InvalidJiraFieldType.Status,
      invalidValue: status,
      validValues: jiraMetadata.statuses(),
    })
  }
  return Errors.makeResult<string>(normalizedStatus)
}

const getIssueType = async (
  context: Context,
  messageContext: MessageContext,
  issueType: string
): Promise<Errors.ResultOrError<
  string | undefined,
  | Errors.InvalidJiraFieldError
  | Errors.JirabotNotEnabledError
  | Errors.UnknownError
>> => {
  if (!issueType) {
    return Errors.makeResult(undefined)
  }
  const jiraMetadataRet = await Jira.getJiraMetadata(
    context,
    messageContext.teamName,
    messageContext.senderUsername
  )
  if (jiraMetadataRet.type === Errors.ReturnType.Error) {
    return jiraMetadataRet
  }
  const jiraMetadata = jiraMetadataRet.result
  const normalizedIssueType = jiraMetadata.normalizeIssueType(issueType)
  if (!normalizedIssueType) {
    return Errors.makeError({
      type: Errors.ErrorType.InvalidJiraField,
      fieldType: Errors.InvalidJiraFieldType.IssueType,
      invalidValue: issueType,
      validValues: jiraMetadata.issueTypes(),
    })
  }
  return Errors.makeResult<string>(normalizedIssueType)
}

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
  conversationId: kbMessage.conversationId,
  messageID: kbMessage.id,
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
  logger.debug({msg: 'got message', messageContext})
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
      const getIssueTypeRet = await getIssueType(
        context,
        messageContext,
        fields[2]
      )
      let issueType: string | undefined = undefined
      if (getIssueTypeRet.type === Errors.ReturnType.Error) {
        if (getIssueTypeRet.error.type !== Errors.ErrorType.InvalidJiraField) {
          Errors.reportErrorAndReplyChat(
            context,
            messageContext,
            getIssueTypeRet.error
          )
          return undefined
        }
        // Ignore the InvalidJiraField error and assume fields[2] is not intended
        // to be an issueType.
      } else {
        issueType = getIssueTypeRet.result
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

      const getProjectRet = await getProject(
        context,
        messageContext,
        args.in,
        true
      )
      if (getProjectRet.type === Errors.ReturnType.Error) {
        Errors.reportErrorAndReplyChat(
          context,
          messageContext,
          getProjectRet.error
        )
        return undefined
      }

      const assignee = args.assignee
        ? args.assignee.replace(/^@+/, '')
        : args.for && args.for.replace(/^@+/, '')

      return {
        context: messageContext,
        type: BotMessageType.Create,
        name: Utils.linebreaksToSpaces(rest[0]),
        project: getProjectRet.result,
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

      const getProjectRet = await getProject(
        context,
        messageContext,
        args.in,
        false
      )
      if (getProjectRet.type === Errors.ReturnType.Error) {
        Errors.reportErrorAndReplyChat(
          context,
          messageContext,
          getProjectRet.error
        )
        return undefined
      }

      const assignee = args.assignee && args.assignee.replace(/^@+/, '')

      const getStatusRet = await getStatus(context, messageContext, args.status)
      if (getStatusRet.type === Errors.ReturnType.Error) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: getStatusRet.error,
        }
      }
      args.status = getStatusRet.result

      return {
        context: messageContext,
        type: BotMessageType.Search,
        query: Utils.linebreaksToSpaces(rest.join(' ')),
        project: getProjectRet.result,
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
          if (toSetName && toSetName !== 'jiraHost') {
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
            toSet: toSetName && {
              name: toSetName,
              value: toSetValue,
            },
          }
        case ConfigType.Channel:
          if (toSetName && !['defaultNewIssueProject'].includes(toSetName)) {
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
            toSet: toSetName && {
              name: toSetName,
              value: toSetValue,
            },
          }
      }
    }
    case 'feed': {
      switch (fields[2]) {
        case undefined:
        case 'list':
          if (fields[3] === 'all') {
            return {
              context: messageContext,
              type: BotMessageType.Feed,
              feedMessageType: FeedMessageType.List,
              allChannelsInTeam: true,
            }
          }
          return {
            context: messageContext,
            type: BotMessageType.Feed,
            feedMessageType: FeedMessageType.List,
            allChannelsInTeam: false,
          }
        case 'subscribe':
          const getProjectRet = await getProject(
            context,
            messageContext,
            fields[3],
            true
          )
          if (getProjectRet.type === Errors.ReturnType.Error) {
            Errors.reportErrorAndReplyChat(
              context,
              messageContext,
              getProjectRet.error
            )
            return undefined
          }
          const project = getProjectRet.result

          if (!project) {
            return {
              context: messageContext,
              type: BotMessageType.Unknown,
              error: `subscribe command requires a project name`,
            }
          }

          return {
            context: messageContext,
            type: BotMessageType.Feed,
            feedMessageType: FeedMessageType.Subscribe,
            project,
            withUpdates: fields[4] === 'with' && fields[5] === 'updates',
          }
        case 'unsubscribe':
          if (!fields[3]) {
            return {
              context: messageContext,
              type: BotMessageType.Unknown,
              error: `subscribe command requires a ID. Use \`!jira feed list\` to see active subscriptions.`,
            }
          }
          const subscriptionID = Number.parseInt(fields[3])
          if (subscriptionID === NaN) {
            return {
              context: messageContext,
              type: BotMessageType.Unknown,
              error: `unexpected ID: ${fields[3]}. ID should be a whole number.`,
            }
          }
          return {
            context: messageContext,
            type: BotMessageType.Feed,
            feedMessageType: FeedMessageType.Unsubscribe,
            subscriptionID: subscriptionID,
          }
        default:
          return {
            context: messageContext,
            type: BotMessageType.Unknown,
            error: `unknown feed command ${fields[2]}`,
          }
      }
    }
    case 'debug': {
      if (!context.botConfig._adminsSet.has(messageContext.senderUsername)) {
        return {
          context: messageContext,
          type: BotMessageType.Unknown,
          error: 'You do not have the permission to use this command.',
        }
      }

      switch (fields[2]) {
        case undefined:
          return {
            context: messageContext,
            type: BotMessageType.Unknown,
            error: `invalid debug command`,
          }
        case 'logSend':
        case 'logsend':
          return {
            context: messageContext,
            type: BotMessageType.Debug,
            debugType: DebugType.LogSend,
          }
        case 'pprof':
          switch (fields[3]) {
            case 'trace':
            case 'cpu':
            case 'heap':
              const matches = (fields[4] ?? '').match(/^(\d+)s$/)
              if (fields[3] !== 'heap' && !matches) {
                return {
                  context: messageContext,
                  type: BotMessageType.Unknown,
                  error: `invalid debug pprof duration ${fields[4]}`,
                }
              }
              return {
                context: messageContext,
                type: BotMessageType.Debug,
                debugType: DebugType.Pprof,
                pprofType: fields[3],
                duration: matches
                  ? Number.parseInt(matches[1]) * 1000
                  : undefined,
              }
            default:
              return {
                context: messageContext,
                type: BotMessageType.Unknown,
                error: `unknown debug pprof type ${fields[3]}`,
              }
          }
        default:
          return {
            context: messageContext,
            type: BotMessageType.Unknown,
            error: `unknown debug command ${fields[2]}`,
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
