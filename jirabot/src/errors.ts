import * as Message from './message'
import {Context} from './context'

export enum ReturnType {
  Ok = 'ok',
  Error = 'error',
}

export type Result<T> = Readonly<{type: ReturnType.Ok; result: T}>
export type Error<E extends Errors> = Readonly<{
  type: ReturnType.Error
  error: E
}>

// functions returning this should never throw
export type ResultOrError<T, E extends Errors> = Result<T> | Error<E>

export enum ErrorType {
  Unknown = 'unknown',
  UnknownParam = 'unknown-param',
  DisabledProject = 'disabled-project',
  MissingProject = 'missing-project',
  ChannelNotConfigured = 'channel-not-configured',
  KVStoreRevision = 'kvstore-revision',
  KVStoreNotFound = 'kvstore-not-found',
}

export type UnknownError = {
  type: ErrorType.Unknown
  description: string
}

export type UnknownParamError = {
  type: ErrorType.UnknownParam
  paramName: string
}

export type DisabledProjectError = {
  type: ErrorType.DisabledProject
  projectName: string
}

export type MissingProjectError = {
  type: ErrorType.MissingProject
}

export type ChannelNotConfiguredError = {
  type: ErrorType.ChannelNotConfigured
}

export type KVStoreRevisionError = {
  type: ErrorType.KVStoreRevision
}

export type KVStoreNotFoundError = {
  type: ErrorType.KVStoreNotFound
}

export type Errors =
  | UnknownError
  | UnknownParamError
  | DisabledProjectError
  | MissingProjectError
  | ChannelNotConfiguredError
  | KVStoreNotFoundError
  | KVStoreRevisionError

export const makeResult = <T>(result: T): Result<T> => ({
  type: ReturnType.Ok,
  result,
})

export const makeError = <E extends Errors>(error: E): Error<E> => ({
  type: ReturnType.Error,
  error,
})

export const channelNotConfiguredError: Error<ChannelNotConfiguredError> = makeError<
  ChannelNotConfiguredError
>({type: ErrorType.ChannelNotConfigured})
export const kvStoreRevisionError: Error<KVStoreRevisionError> = makeError<
  KVStoreRevisionError
>({type: ErrorType.KVStoreRevision})
export const kvStoreNotFoundError: Error<KVStoreNotFoundError> = makeError<
  KVStoreNotFoundError
>({type: ErrorType.KVStoreNotFound})
export const missingProjectError: Error<MissingProjectError> = makeError<
  MissingProjectError
>({type: ErrorType.MissingProject})

export const reportErrorAndReplyChat = (
  context: Context,
  messageContext: Message.MessageContext,
  error: Errors
): Promise<any> => {
  switch (error.type) {
    case ErrorType.Unknown:
      console.warn('unknown error', error, messageContext)
      return context.bot.chat.send(messageContext.chatChannel, {
        body: 'Whoops. Something happened and your command has failed.',
      })
    case ErrorType.UnknownParam:
      return context.bot.chat.send(messageContext.chatChannel, {
        body: `unknown param ${error.paramName}`,
      })
    case ErrorType.DisabledProject:
      return context.bot.chat.send(messageContext.chatChannel, {
        body: `project "${error.projectName}" is disabled in this channel`,
      })
    case ErrorType.MissingProject:
      return context.bot.chat.send(messageContext.chatChannel, {
        body:
          'You need to specify a prject name for this command. You can also `!jira config channel defaultNewIssueProject <default-project> to set a default one for this channel.',
      })
    case ErrorType.ChannelNotConfigured:
      return context.bot.chat.send(messageContext.chatChannel, {
        body:
          'Jira is not configured for this channel. See `!jira config channel`',
      })

    case ErrorType.KVStoreRevision:
    case ErrorType.KVStoreNotFound:
      return null
  }
}
