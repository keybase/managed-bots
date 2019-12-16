import JiraClient from 'jira-connector'
import crypto from 'crypto'
import * as Constants from './constants'
import * as Errors from './errors'
import * as Configs from './configs'

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

const step1 = (host: string, consumerKey: string, privateKey: string) =>
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
        host,
        oauth: {
          consumer_key: consumerKey,
          private_key: privateKey,
          callback_url: `http://localhost:8080${Constants.jiraOauthCallbackPathname}`,
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
  teamJiraConfig: Configs.TeamJiraConfig,
  onAuthUrl: (url: string) => void
): Promise<Errors.ResultOrError<
  OauthResult,
  Errors.UnknownError | Errors.TimeoutError
>> => {
  const step1ResultOrError = await step1(
    teamJiraConfig.jiraHost,
    teamJiraConfig.jiraAuth.consumerKey,
    teamJiraConfig.jiraAuth.privateKey
  )
  if (step1ResultOrError.type === Errors.ReturnType.Error) {
    return step1ResultOrError
  }
  const res1 = step1ResultOrError.result

  if (
    typeof res1.token !== 'string' ||
    typeof res1.token_secret !== 'string' ||
    typeof res1.url !== 'string'
  ) {
    return Errors.makeUnknownError(new Error('unexpected response from jira'))
  }

  onAuthUrl(res1.url)

  const waitForJiraCallbackResultOrError = await waitForJiraCallback(res1.token)
  if (waitForJiraCallbackResultOrError.type === Errors.ReturnType.Error) {
    return waitForJiraCallbackResultOrError
  }
  const tokenCallbackData = waitForJiraCallbackResultOrError.result

  const accessTokenResultOrError = await getAccessToken(
    teamJiraConfig.jiraHost,
    teamJiraConfig.jiraAuth.consumerKey,
    teamJiraConfig.jiraAuth.privateKey,
    res1.token_secret,
    tokenCallbackData
  )
  if (accessTokenResultOrError.type === Errors.ReturnType.Error) {
    return accessTokenResultOrError
  }

  return Errors.makeResult({
    accessToken: accessTokenResultOrError.result,
    tokenSecret: res1.token_secret,
  })
}

export type JiraLinkDetails = {
  privateKey: string
  publicKey: string
  consumerKey: string
}

export const generarteNewJiraLinkDetails = async (): Promise<Errors.ResultOrError<
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
    new Promise<string>((resolve, reject) =>
      crypto.randomBytes(16, (err, buf) => {
        err ? reject(err) : resolve(`keybase-jirabot-${buf.toString('hex')}`)
      })
    ),
  ])
    .then(([{privateKey, publicKey}, consumerKey]) =>
      Errors.makeResult({
        privateKey,
        publicKey,
        consumerKey,
      })
    )
    .catch(err => Errors.makeUnknownError(err))
