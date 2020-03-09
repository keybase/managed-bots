import * as Message from './message'
import {Context} from './context'
import logger from './logger'
import * as Utils from './utils'
import {startAuth} from './cmd-auth'

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
  Timeout = 'timeout',
  UnknownParam = 'unknown-param',
  InvalidJiraField = 'invalid-field',
  MissingProject = 'missing-project',
  KVStoreRevision = 'kvstore-revision',
  KVStoreNotFound = 'kvstore-not-found',
  JirabotNotEnabled = 'jirabot-not-enabled',
  JiraNoPermission = 'jira-no-permission',
}

export type UnknownError = {
  type: ErrorType.Unknown
  originalError: any
  description: string
}

export const makeUnknownError = (err: any): Error<UnknownError> =>
  makeError({
    type: ErrorType.Unknown,
    originalError: err,
    description:
      err && typeof err.toString === 'function' ? err.toString() : '',
  })

export type TimeoutError = {
  type: ErrorType.Timeout
  description: string
}

export type UnknownParamError = {
  type: ErrorType.UnknownParam
  paramName: string
}

export enum InvalidJiraFieldType {
  Project = 'project',
  IssueType = 'issueType',
  Status = 'status',
}

const invalidJiraFieldTypeToString = (
  fieldType: InvalidJiraFieldType,
  capitalizeFirstChar?: boolean
): string => {
  switch (fieldType) {
    case InvalidJiraFieldType.Project:
      return capitalizeFirstChar ? 'Project' : 'project'
    case InvalidJiraFieldType.IssueType:
      return capitalizeFirstChar ? 'Issue type' : 'issue type'
    case InvalidJiraFieldType.Status:
      return capitalizeFirstChar ? 'Status' : 'status'
  }
}

export type InvalidJiraFieldError = {
  type: ErrorType.InvalidJiraField
  fieldType: InvalidJiraFieldType
  invalidValue: string
  validValues: Array<string>
}

export type MissingProjectError = {
  type: ErrorType.MissingProject
}

export type KVStoreRevisionError = {
  type: ErrorType.KVStoreRevision
}

export type KVStoreNotFoundError = {
  type: ErrorType.KVStoreNotFound
}

export type JirabotNotEnabledError = Readonly<{
  type: ErrorType.JirabotNotEnabled
  notEnabledType: 'team' | 'user'
}>

export type JiraNoPermissionError = {
  type: ErrorType.JiraNoPermission
}

export const JirabotNotEnabledForTeamError: JirabotNotEnabledError = {
  type: ErrorType.JirabotNotEnabled,
  notEnabledType: 'team',
} as const

export const JirabotNotEnabledForUserError: JirabotNotEnabledError = {
  type: ErrorType.JirabotNotEnabled,
  notEnabledType: 'user',
} as const

export type Errors =
  | UnknownError
  | TimeoutError
  | UnknownParamError
  | InvalidJiraFieldError
  | MissingProjectError
  | KVStoreNotFoundError
  | KVStoreRevisionError
  | JirabotNotEnabledError
  | JiraNoPermissionError

export const makeResult = <T>(result: T): Result<T> => ({
  type: ReturnType.Ok,
  result,
})

export const makeError = <E extends Errors>(error: E): Error<E> => ({
  type: ReturnType.Error,
  error,
})

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
      logger.warn({msg: 'unknown error', error, messageContext})
      return Utils.replyToMessageContext(
        context,
        messageContext,
        'Whoops. Something happened and your command has failed.'
      )
    case ErrorType.UnknownParam:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        `unknown param ${error.paramName}`
      )
    case ErrorType.InvalidJiraField:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        `${invalidJiraFieldTypeToString(error.fieldType, true)} "${
          error.invalidValue
        }" is invalid. Your Jira projects are: ${Utils.humanReadableArray(
          error.validValues
        )}`
      )
    case ErrorType.MissingProject:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        'You need to specify a project name for this command. You can also `!jira config channel defaultNewIssueProject <default-project>` to set a default one for this channel.'
      )
    case ErrorType.Timeout:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        `An operation has timed out: ${error.description}`
      )
    case ErrorType.JirabotNotEnabled:
      switch (error.notEnabledType) {
        case 'team':
          return Utils.replyToMessageContext(
            context,
            messageContext,
            'This team has not been configured for jirabot. Use `!jira config team jiraHost <domain>` to enable jirabot for this team.'
          )
        case 'user':
          return Utils.replyToMessageContext(
            context,
            messageContext,
            'You have not given Jirabot permission to access your Jira account. I will start the authorization process for you.'
          ).then(() => startAuth(context, messageContext))
        default:
          let _: never = error.notEnabledType
      }
    case ErrorType.JiraNoPermission:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        `You do not have permission to perform this operation on Jira. Maybe ask a Jira admin to do it?`
      )

    case ErrorType.KVStoreRevision:
    case ErrorType.KVStoreNotFound:
      return null
  }
}
