import ChatTypes from 'keybase-bot/lib/types/chat1'
import * as Message from './message'
import * as Errors from './errors'
import CmdSearch from './cmd-search'
import CmdComment from './cmd-comment'
import CmdAuth from './cmd-auth'
import reacji from './reacji'
import CmdNew from './cmd-new'
import CmdConfig from './cmd-config'
import CmdFeed from './cmd-feed'
import {Context} from './context'
import logger from './logger'
import * as Utils from './utils'

const reportError = (context: Context, parsedMessage: Message.UnknownMessage) =>
  parsedMessage.error && typeof parsedMessage.error !== 'string'
    ? Errors.reportErrorAndReplyChat(
        context,
        parsedMessage.context,
        parsedMessage.error
      )
    : Utils.replyToMessageContext(
        context,
        parsedMessage.context,
        parsedMessage.error
          ? `Invalid command: ${parsedMessage.error}`
          : 'Unknown command'
      )

const reactAck = (
  context: Context,
  messageContext: Message.MessageContext,
  id: number
) => context.bot.chat.react(messageContext.conversationId, id, ':eyes:')

const reactDone = (
  context: Context,
  messageContext: Message.MessageContext,
  id: number
) =>
  context.bot.chat.react(
    messageContext.conversationId,
    id,
    ':white_check_mark:'
  )

const reactFail = (
  context: Context,
  messageContext: Message.MessageContext,
  id: number
) => context.bot.chat.react(messageContext.conversationId, id, ':x:')

const onMessage = async (
  context: Context,
  kbMessage: ChatTypes.MsgSummary
): Promise<void> => {
  try {
    const parsedMessage = await Message.parseMessage(context, kbMessage)
    if (!parsedMessage) {
      // not a jirabot message
      return
    }
    logger.debug({msg: 'parsed message', messageContext: parsedMessage.context})
    context.stathat.postCount(`messageType ${parsedMessage.type}`, 1)
    switch (parsedMessage.type) {
      case Message.BotMessageType.Unknown:
        reportError(context, parsedMessage)
        return
      case Message.BotMessageType.Search: {
        reactAck(context, parsedMessage.context, kbMessage.id)
        const {type} = await CmdSearch(context, parsedMessage)
        type === Errors.ReturnType.Ok
          ? reactDone(context, parsedMessage.context, kbMessage.id)
          : reactFail(context, parsedMessage.context, kbMessage.id)
        return
      }
      case Message.BotMessageType.Comment: {
        reactAck(context, parsedMessage.context, kbMessage.id)
        const {type} = await CmdComment(context, parsedMessage)
        type === Errors.ReturnType.Ok
          ? reactDone(context, parsedMessage.context, kbMessage.id)
          : reactFail(context, parsedMessage.context, kbMessage.id)
        return
      }
      case Message.BotMessageType.Reacji:
        reacji(context, parsedMessage)
        return
      case Message.BotMessageType.Create: {
        reactAck(context, parsedMessage.context, kbMessage.id)
        const {type} = await CmdNew(context, parsedMessage)
        type === Errors.ReturnType.Ok
          ? reactDone(context, parsedMessage.context, kbMessage.id)
          : reactFail(context, parsedMessage.context, kbMessage.id)
        return
      }
      case Message.BotMessageType.Config: {
        reactAck(context, parsedMessage.context, kbMessage.id)
        const {type} = await CmdConfig(context, parsedMessage)
        type === Errors.ReturnType.Ok
          ? reactDone(context, parsedMessage.context, kbMessage.id)
          : reactFail(context, parsedMessage.context, kbMessage.id)
        return
      }
      case Message.BotMessageType.Auth: {
        reactAck(context, parsedMessage.context, kbMessage.id)
        const {type} = await CmdAuth(context, parsedMessage)
        type === Errors.ReturnType.Ok
          ? reactDone(context, parsedMessage.context, kbMessage.id)
          : reactFail(context, parsedMessage.context, kbMessage.id)
        return
      }
      case Message.BotMessageType.Feed: {
        reactAck(context, parsedMessage.context, kbMessage.id)
        const {type} = await CmdFeed(context, parsedMessage)
        type === Errors.ReturnType.Ok
          ? reactDone(context, parsedMessage.context, kbMessage.id)
          : reactFail(context, parsedMessage.context, kbMessage.id)
        return
      }
      default:
        let _: never = parsedMessage
    }
  } catch (err) {
    // otherwise keybase-bot seems to swallow exceptions
    logger.error(err)
  }
}

const commands = [
  {
    name: 'jira new',
    description: 'make a Jira ticket',
    usage: `[issue-type] [in <PROJECT>] [for|assignee <kb-username>] "multi word summary" <description>`,
    title: 'Create a Jira ticket',
    body:
      'Examples:\n\n' +
      `!jira new "blah ticket" blah is broken!\n` +
      `!jira new _in_ FRONTEND "UI tweaks for menu" margin should be 16px on desktop and 24px on mobile\n` +
      `!jira new bug _in_ frontend _for_ @songgao "fix fs offline bug" app thinks it's offline when it's not\n`,
  },
  {
    name: 'jira search',
    description: 'search for Jira tickets',
    usage: `[in <PROJECT>] [assignee <kb-username>] <query>`,
    title: 'Search for Jira tickets',
    body:
      'Examples:\n\n' +
      `!jira search rake in the lake\n` +
      `!jira search _in_ DESIGN _assignee_ @cecileb bot popup\n` +
      `!jira search _in_ FRONTEND _status_ "to do" offline mode\n`,
  },
  {
    name: 'jira comment',
    description: `Comment on a Jira tickets.`,
    usage: `on <ticket-key> <content>`,
    title: 'Comment on a Jira ticket',
    body:
      'Examples:\n\n' + `!jira comment on TRIAGE-1024 this is already fixed\n`,
  },
  {
    name: 'jira config',
    description: `Show or change jirabot configuration for this team or channel`,
    usage: `team [<param-name> <param-value>] | channel [<param-name> <param-value>]`,
    title: 'Jirabot Configuration',
    body:
      'Examples:\n\n' +
      `!jira config team jiraHost foo.atlassian.net\n` +
      `!jira config channel\n` +
      // `!jira config team\n`+
      `!jira config channel defaultNewIssueProject DESIGN\n`,
  },
  {
    name: 'jira auth',
    description: `Connect Jirabot to your Jira account`,
    usage: ``,
    title: 'Jira Authorization',
  },
  {
    name: 'jira feed',
    description: `Subscribe to Jira feed and receive messages on Keybase about Jira activities.`,
    usage: `list [all] | subscribe <project|'all'> [with updates] | unsubscribe <id>`,
    title: 'Subscribe to Jira feed',
    body:
      'Examples:\n\n' +
      '!jira list\n' +
      '!jira list all\n' +
      '!jira subscribe all\n' +
      '!jira subscribe design\n' +
      '!jira subscribe frontend with updates\n' +
      '!jira unsubscribe 123',
  },
]

const advertisements = [
  {
    type: 'public',
    commands: commands.map(({name, description, usage, title, body}) => ({
      name,
      description,
      usage,
      extendedDescription: {
        title,
        desktopBody: body,
        mobileBody: body,
      },
    })),
  },
]

const onNewConversation = async (
  context: Context,
  convSummary: ChatTypes.ConvSummary
) => {
  if (
    !convSummary.isDefaultConv ||
    convSummary.creatorInfo?.username === context.botConfig.keybase.username
  ) {
    // only say hi in #general; don't say hi in bot-created conversations.
    return
  }
  logger.info({msg: 'onNewConversation', channel: convSummary.channel})
  const teamJiraConfigRet = await context.configs.getTeamJiraConfig(
    convSummary.channel.name
  )
  if (teamJiraConfigRet.type === Errors.ReturnType.Ok) {
    await context.bot.chat.send(convSummary.id, {
      body: `Manage your Jira workflow without leaving the Keybase app. Your Jira admin has configured this team for ${teamJiraConfigRet.result.config.jiraHost}. Type \`!jira\` to see a list of supported commands.`,
    })
  } else if (
    teamJiraConfigRet.error.type === Errors.ErrorType.KVStoreNotFound
  ) {
    await context.bot.chat.send(convSummary.id, {
      body: `Manage your Jira workflow without leaving the Keybase app. Get started by making an application link on Jira: \`!jira config team\``,
    })
  } else {
    logger.warn({msg: 'onNewConversation', error: teamJiraConfigRet.error})
  }
}

export default (context: Context) => {
  context.bot.chat.advertiseCommands({
    alias: 'Jira',
    advertisements,
  })
  context.bot.chat.watchAllChannelsForNewMessages(message =>
    onMessage(context, message)
  )
  context.bot.chat.watchForNewConversation(convSummary =>
    onNewConversation(context, convSummary)
  )
}
