import * as Message from './message'
import {Context} from './context'
import * as Errors from './errors'
import fs from 'fs'
import logger from './logger'

export default async (
  context: Context,
  parsedMessage: Message.DebugMessage
): Promise<Errors.ResultOrError<undefined, undefined>> => {
  if (!context.botConfig._adminsSet.has(parsedMessage.context.senderUsername)) {
    return Errors.makeError(undefined)
  }

  switch (parsedMessage.debugType) {
    case Message.DebugType.LogSend:
      await context.bot.logSend()
      return Errors.makeResult(undefined)
    case Message.DebugType.Pprof:
      const pprofFilePath = await context.bot.pprof(
        parsedMessage.pprofType,
        parsedMessage.duration
      )
      await context.bot.chat.attach(
        parsedMessage.context.conversationId,
        pprofFilePath
      )
      await new Promise(resolve =>
        fs.unlink(pprofFilePath, error => {
          if (error) {
            logger.warn({msg: 'pprof-unlink', error})
          }
          resolve()
        })
      )
      return Errors.makeResult(undefined)
  }
}
