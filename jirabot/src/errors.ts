import * as Message from "./message";
import { Context } from "./context";
import logger from "./logger";
import * as Utils from "./utils";

export enum ReturnType {
  Ok = "ok",
  Error = "error"
}

export type Result<T> = Readonly<{ type: ReturnType.Ok; result: T }>;
export type Error<E extends Errors> = Readonly<{
  type: ReturnType.Error;
  error: E;
}>;

// functions returning this should never throw
export type ResultOrError<T, E extends Errors> = Result<T> | Error<E>;

export enum ErrorType {
  Unknown = "unknown",
  Timeout = "timeout",
  UnknownParam = "unknown-param",
  DisabledProject = "disabled-project",
  MissingProject = "missing-project",
  KVStoreRevision = "kvstore-revision",
  KVStoreNotFound = "kvstore-not-found",
  JirabotNotEnabled = "jirabot-not-enabled"
}

export type UnknownError = {
  type: ErrorType.Unknown;
  originalError: any;
  description: string;
};

export const makeUnknownError = (err: any): Error<UnknownError> =>
  makeError({
    type: ErrorType.Unknown,
    originalError: err,
    description: err && typeof err.toString === "function" ? err.toString() : ""
  });

export type TimeoutError = {
  type: ErrorType.Timeout;
  description: string;
};

export type UnknownParamError = {
  type: ErrorType.UnknownParam;
  paramName: string;
};

export type DisabledProjectError = {
  type: ErrorType.DisabledProject;
  projectName: string;
};

export type MissingProjectError = {
  type: ErrorType.MissingProject;
};

export type KVStoreRevisionError = {
  type: ErrorType.KVStoreRevision;
};

export type KVStoreNotFoundError = {
  type: ErrorType.KVStoreNotFound;
};

export type JirabotNotEnabledError = Readonly<{
  type: ErrorType.JirabotNotEnabled;
  notEnabledType: "team" | "user";
}>;

export const JirabotNotEnabledForTeamError: JirabotNotEnabledError = {
  type: ErrorType.JirabotNotEnabled,
  notEnabledType: "team"
} as const;

export const JirabotNotEnabledForUserError: JirabotNotEnabledError = {
  type: ErrorType.JirabotNotEnabled,
  notEnabledType: "user"
} as const;

export type Errors =
  | UnknownError
  | TimeoutError
  | UnknownParamError
  | DisabledProjectError
  | MissingProjectError
  | KVStoreNotFoundError
  | KVStoreRevisionError
  | JirabotNotEnabledError;

export const makeResult = <T>(result: T): Result<T> => ({
  type: ReturnType.Ok,
  result
});

export const makeError = <E extends Errors>(error: E): Error<E> => ({
  type: ReturnType.Error,
  error
});

export const kvStoreRevisionError: Error<KVStoreRevisionError> = makeError<
  KVStoreRevisionError
>({ type: ErrorType.KVStoreRevision });
export const kvStoreNotFoundError: Error<KVStoreNotFoundError> = makeError<
  KVStoreNotFoundError
>({ type: ErrorType.KVStoreNotFound });
export const missingProjectError: Error<MissingProjectError> = makeError<
  MissingProjectError
>({ type: ErrorType.MissingProject });

export const reportErrorAndReplyChat = (
  context: Context,
  messageContext: Message.MessageContext,
  error: Errors
): Promise<any> => {
  switch (error.type) {
    case ErrorType.Unknown:
      logger.warn({ msg: "unknown error", error, messageContext });
      return Utils.replyToMessageContext(
        context,
        messageContext,
        "Whoops. Something happened and your command has failed."
      );
    case ErrorType.UnknownParam:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        `unknown param ${error.paramName}`
      );
    case ErrorType.DisabledProject:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        `project "${error.projectName}" is disabled in this channel`
      );
    case ErrorType.MissingProject:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        "You need to specify a prject name for this command. You can also `!jira config channel defaultNewIssueProject <default-project> to set a default one for this channel."
      );
    case ErrorType.Timeout:
      return Utils.replyToMessageContext(
        context,
        messageContext,
        `An operation has timed out: ${error.description}`
      );
    case ErrorType.JirabotNotEnabled:
      switch (error.notEnabledType) {
        case "team":
          return Utils.replyToMessageContext(
            context,
            messageContext,
            "This team has not been configured for jirabot. Use `!jirabot config team jiraHost <domain>` to enable jirabot for this team."
          );
        case "user":
          return Utils.replyToMessageContext(
            context,
            messageContext,
            "You have not given Jirabot permission to access your Jira account. In order to use Jirabot, connect with `!jira auth`."
          );
        default:
          let _: never = error.notEnabledType;
      }

    case ErrorType.KVStoreRevision:
    case ErrorType.KVStoreNotFound:
      return null;
  }
};
