import ChatTypes from 'keybase-bot/lib/types/chat1'
import * as Message from './message'
import * as Errors from './errors'
import CmdSearch from './cmd-search'
import CmdComment from './cmd-comment'
import reacji from './reacji'
import CmdNew from './cmd-new'
import CmdConfig from './cmd-config'
import {Context} from './context'

const reportError = (context: Context, parsedMessage: Message.Message) =>
  context.bot.chat.send(parsedMessage.context.chatChannel, {
    body:
      parsedMessage.type === 'unknown'
        ? `Invalid command: ${parsedMessage.error}`
        : 'Unknown command',
  })

const reactAck = (
  context: Context,
  channel: ChatTypes.ChatChannel,
  id: number
) => context.bot.chat.react(channel, id, ':eyes:')

const reactDone = (
  context: Context,
  channel: ChatTypes.ChatChannel,
  id: number
) => context.bot.chat.react(channel, id, ':white_check_mark:')

const reactFail = (
  context: Context,
  channel: ChatTypes.ChatChannel,
  id: number
) => context.bot.chat.react(channel, id, ':x:')

const onMessage = async (
  context: Context,
  kbMessage: ChatTypes.MsgSummary
): Promise<void> => {
  try {
    // console.debug(kbMessage)
    const parsedMessage = await Message.parseMessage(context, kbMessage)
    console.debug({msg: 'got message', parsedMessage})
    if (!parsedMessage) {
      // not a jirabot message
      return
    }
    switch (parsedMessage.type) {
      case Message.BotMessageType.Unknown:
        reportError(context, parsedMessage)
        return
      case Message.BotMessageType.Search:
        reactAck(context, kbMessage.channel, kbMessage.id)
        CmdSearch(context, parsedMessage)
        return
      case Message.BotMessageType.Comment:
        reactAck(context, kbMessage.channel, kbMessage.id)
        CmdComment(context, parsedMessage)
        return
      case Message.BotMessageType.Reacji:
        reacji(context, parsedMessage)
        return
      case Message.BotMessageType.Create:
        reactAck(context, kbMessage.channel, kbMessage.id)
        CmdNew(context, parsedMessage)
        return
      case Message.BotMessageType.Config:
        reactAck(context, kbMessage.channel, kbMessage.id)
        const {type} = await CmdConfig(context, parsedMessage)
        type === Errors.ReturnType.Ok
          ? reactDone(context, kbMessage.channel, kbMessage.id)
          : reactFail(context, kbMessage.channel, kbMessage.id)
        return
      default:
        console.error({error: 'we forgot to handle a case in onMessage'})
        return
    }
  } catch (err) {
    // otherwise keybase-bot seems to swallow exceptions
    console.error(err)
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
      `!jira config team\n` +
      `!jira config channel\n` +
      // `!jira config team\n`+
      `!jira config channel defaultNewIssueProject DESIGN\n` +
      `!jira config channel enabledProjects DESIGN,FRONTEND\n` +
      `!jira config channel enabledProjects *\n`,
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

export default (context: Context) => {
  context.bot.chat.advertiseCommands({
    alias: 'Jira',
    advertisements,
  })
  context.bot.chat.watchAllChannelsForNewMessages(message =>
    onMessage(context, message)
  )
}
