import JiraClient from 'jira-connector'
import crypto from 'crypto'
import * as Constants from './constants'
import * as Errors from './errors'
import * as Configs from './configs'
import {Context} from './context'
import * as Utils from './utils'

export type OauthResult = Readonly<{
  accessToken: string
  tokenSecret: string
}>

type TokenCallbackData = {
  oauthToken: string
  oauthVerifier: string
}

const tokenCallbacks = new Map<string, (data: TokenCallbackData) => void>()

export const onJiraCallback = (oauthToken: string, oauthVerifier: string) => {
  const tokenCallback = tokenCallbacks.get(oauthToken)
  tokenCallback && tokenCallback({oauthToken, oauthVerifier})
}

const step1 = (
  jiraHost: string,
  consumerKey: string,
  privateKey: string,
  httpAddressPrefix: string
) =>
  new Promise<
    Errors.ResultOrError<
      {
        token: string
        token_secret: string
        url: string
      },
      Errors.UnknownError
    >
  >(resolve => {
    JiraClient.oauth_util.getAuthorizeURL(
      {
        host: jiraHost,
        oauth: {
          consumer_key: consumerKey,
          private_key: privateKey,
          callback_url: `${httpAddressPrefix}${Constants.jiraOauthCallbackPathname}`,
        },
      },
      (error: any, oauth: any) => {
        error
          ? resolve(Errors.makeUnknownError(error))
          : resolve(Errors.makeResult(oauth))
      }
    )
  })

const waitForJiraCallback = (
  oauthToken: string
): Promise<Errors.ResultOrError<TokenCallbackData, Errors.TimeoutError>> =>
  new Promise(resolve => {
    const timeoutId = setTimeout(() => {
      tokenCallbacks.delete(oauthToken)
      resolve(
        Errors.makeError({
          type: Errors.ErrorType.Timeout,
          description: 'jira permission was not granted',
        })
      )
    }, Constants.jiraCallbackTimeout)
    tokenCallbacks.set(oauthToken, data => {
      tokenCallbacks.delete(oauthToken)
      clearTimeout(timeoutId)
      resolve(Errors.makeResult(data))
    })
  })

const getAccessToken = (
  host: string,
  consumerKey: string,
  privateKey: string,
  tokenSecret: string,
  tokenCallbackData: TokenCallbackData
) =>
  new Promise<Errors.ResultOrError<string, Errors.UnknownError>>(resolve => {
    JiraClient.oauth_util.swapRequestTokenWithAccessToken(
      {
        host,
        oauth: {
          token: tokenCallbackData.oauthToken,
          token_secret: tokenSecret,
          oauth_verifier: tokenCallbackData.oauthVerifier,
          consumer_key: consumerKey,
          private_key: privateKey,
        },
      },
      (error: any, accessToken: string) =>
        error
          ? resolve(Errors.makeUnknownError(error))
          : resolve(Errors.makeResult(accessToken))
    )
  })

export const doOauth = async (
  context: Context,
  teamJiraConfig: Configs.TeamJiraConfig,
  onAuthUrl: (url: string) => void
): Promise<Errors.ResultOrError<
  OauthResult,
  Errors.UnknownError | Errors.TimeoutError
>> => {
  const step1Ret = await step1(
    teamJiraConfig.jiraHost,
    teamJiraConfig.jiraAuth.consumerKey,
    teamJiraConfig.jiraAuth.privateKey,
    context.botConfig.httpAddressPrefix
  )
  if (step1Ret.type === Errors.ReturnType.Error) {
    return step1Ret
  }
  const res1 = step1Ret.result

  if (
    typeof res1.token !== 'string' ||
    typeof res1.token_secret !== 'string' ||
    typeof res1.url !== 'string'
  ) {
    return Errors.makeUnknownError(new Error('unexpected response from jira'))
  }

  onAuthUrl(res1.url)

  const waitForJiraCallbackRet = await waitForJiraCallback(res1.token)
  if (waitForJiraCallbackRet.type === Errors.ReturnType.Error) {
    return waitForJiraCallbackRet
  }
  const tokenCallbackData = waitForJiraCallbackRet.result

  const accessTokenRet = await getAccessToken(
    teamJiraConfig.jiraHost,
    teamJiraConfig.jiraAuth.consumerKey,
    teamJiraConfig.jiraAuth.privateKey,
    res1.token_secret,
    tokenCallbackData
  )
  if (accessTokenRet.type === Errors.ReturnType.Error) {
    return accessTokenRet
  }

  return Errors.makeResult({
    accessToken: accessTokenRet.result,
    tokenSecret: res1.token_secret,
  })
}

export type JiraLinkDetails = {
  privateKey: string
  publicKey: string
  consumerKey: string
}

export const generateNewJiraLinkDetails = async (): Promise<Errors.ResultOrError<
  JiraLinkDetails,
  Errors.UnknownError
>> =>
  Promise.all([
    new Promise<{privateKey: string; publicKey: string}>((resolve, reject) =>
      crypto.generateKeyPair(
        'rsa',
        {
          modulusLength: 2048,
          publicKeyEncoding: {
            type: 'spki',
            format: 'pem',
          },
          privateKeyEncoding: {
            type: 'pkcs1',
            format: 'pem',
          },
        },
        (err, publicKey, privateKey) => {
          err
            ? reject(err)
            : resolve({
                privateKey,
                publicKey,
              })
        }
      )
    ),
    Utils.randomString('keybase-jirabot'),
  ])
    .then(([{privateKey, publicKey}, consumerKey]) =>
      Errors.makeResult({
        privateKey,
        publicKey,
        consumerKey,
      })
    )
    .catch(err => Errors.makeUnknownError(err))
