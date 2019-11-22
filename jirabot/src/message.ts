import ChatTypes from 'keybase-bot/lib/types/chat1'
// @ts-ignore
import * as Utils from './utils'
import {Context} from './context'

export enum BotMessageType {
  Unknown = 'unknown',
  Help = 'help',
  Create = 'create',
  Search = 'search',
  Comment = 'comment',
  Reacji = 'reacji',
}

type UnknownMessage = {
  type: BotMessageType.Unknown
  error?: string
}

type HelpMessage = {
  type: BotMessageType.Help
}

export type CreateMessage = {
  from: string
  type: BotMessageType.Create
  name: string
  project: string
  assignee: string
  description: string
  issueType: string
}

export type SearchMessage = {
  from: string
  type: BotMessageType.Search
  query: string
  project: string
  status: string
  assignee: string
}

export type CommentMessage = {
  from: string
  type: BotMessageType.Comment
  ticket: string
  comment: string
}

export type ReacjiMessage = {
  from: string
  type: BotMessageType.Reacji
  reactToID: number
  emoji: string
}

export type Message = UnknownMessage | HelpMessage | SearchMessage | CommentMessage | ReacjiMessage | CreateMessage

const cmdRE = new RegExp(/(?:!jira)\s+(\S+)(?:\s+(\S+))?(?:\s+(.*))?/)

const getTextMessage = (message: ChatTypes.MsgSummary): string | undefined => {
  if (!message || !message.content) {
    return undefined
  }
  if (message.content.type === 'text' && message.content.text && typeof message.content.text.body === 'string') {
    return message.content.text.body
  }
  if (message.content.type === 'edit' && message.content.edit && typeof message.content.edit.body === 'string') {
    return message.content.edit.body
  }
  return undefined
}

const checkProjectError = (context: Context, project: string): string | undefined =>
  project && !context.config.jira.projects.includes(project)
    ? `invalid project: ${project} is not one of ${Utils.humanReadableArray(context.config.jira.projects)}`
    : undefined

const checkStatusError = (context: Context, status: string): string | undefined =>
  status && !context.config.jira.status.includes(status)
    ? `invalid status: ${status} is not one of ${Utils.humanReadableArray(context.config.jira.status)}`
    : undefined

const checkAssigneeError = (context: Context, assignee: string): string | undefined =>
  assignee && !context.config.jira.usernameMapper[assignee]
    ? `invalid assignee: ${assignee} is not one of ${Utils.humanReadableArray(Object.keys(context.config.jira.usernameMapper))}`
    : undefined

const checkIssueTypeError = (context: Context, issueType: string): string | undefined =>
  issueType && !context.config.jira.issueTypes.includes(issueType)
    ? `invalid issueType: ${issueType} is not one of ${Utils.humanReadableArray(context.config.jira.issueTypes)}`
    : undefined

function extractArgsAfterCommand<K extends string>(
  fields: Array<string>,
  keys: Set<K>
): {
  args: Partial<Record<K, string>>
  rest: Array<string>
} {
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

const newArgs = new Set(['in', 'for', 'assignee'])
const searchArgs = new Set(['in', 'assignee', 'status'])
const commentArgs = new Set(['on'])

export const parseMessage = (context: Context, kbMessage: ChatTypes.MsgSummary): null | Message => {
  const textBody = getTextMessage(kbMessage)
  if (!textBody) {
    return null
  }

  if (!textBody.startsWith('!jira')) {
    return null
  }

  const fields = Utils.split2(textBody)

  switch (fields[1]) {
    case 'new': {
      const issueTypeFromInputCaseMapped = context.config.jira._issueTypeInsensitiveToIssueType(fields[2])
      const issueType = context.config.jira.issueTypes.includes(issueTypeFromInputCaseMapped) ? issueTypeFromInputCaseMapped : ''

      const issueTypeError = checkIssueTypeError(context, issueType)
      if (issueTypeError) {
        return {type: BotMessageType.Unknown, error: issueTypeError}
      }

      const {args, rest} = extractArgsAfterCommand(fields.slice(issueType ? 3 : 2), newArgs)
      if (!args.in) {
        return {type: BotMessageType.Unknown, error: '`!jira new` needs the `in` argument for the project key'}
      }
      if (rest.length < 1) {
        return {type: BotMessageType.Unknown, error: '`!jira new` needs at least a summary for the ticket'}
      }

      const projectError = checkProjectError(context, args.in)
      if (projectError) {
        return {type: BotMessageType.Unknown, error: projectError}
      }

      const assignee = args.assignee ? args.assignee.replace(/^@+/, '') : args.for && args.for.replace(/^@+/, '')
      const assigneeError = checkAssigneeError(context, assignee)
      if (assigneeError) {
        return {type: BotMessageType.Unknown, error: assigneeError}
      }

      return {
        from: kbMessage.sender.username,
        type: BotMessageType.Create,
        name: rest[0],
        project: args.in,
        assignee,
        description: rest.slice(1).join(' '),
        issueType,
      }
    }
    case 'search': {
      const {args, rest} = extractArgsAfterCommand(fields.slice(2), searchArgs)
      if (rest.length < 1) {
        return {type: BotMessageType.Unknown, error: '`!jira search` needs at least a query'}
      }

      const projectError = checkProjectError(context, args.in)
      if (projectError) {
        return {type: BotMessageType.Unknown, error: projectError}
      }

      const assignee = args.assignee && args.assignee.replace(/^@+/, '')
      const assigneeError = checkAssigneeError(context, assignee)
      if (assigneeError) {
        return {type: BotMessageType.Unknown, error: assigneeError}
      }

      const statusError = checkStatusError(context, args.status)
      if (statusError) {
        return {type: BotMessageType.Unknown, error: statusError}
      }

      return {
        from: kbMessage.sender.username,
        type: BotMessageType.Search,
        query: rest.join(' '),
        project: args.in,
        assignee,
        status: args.status,
      }
    }
    case 'comment': {
      const {args, rest} = extractArgsAfterCommand(fields.slice(2), commentArgs)
      if (rest.length < 1) {
        return {type: BotMessageType.Unknown, error: '`!jira comment` needs a comment to post'}
      }
      return {
        from: kbMessage.sender.username,
        type: BotMessageType.Comment,
        ticket: args.on,
        comment: rest.join(' '),
      }
    }
  }
}
