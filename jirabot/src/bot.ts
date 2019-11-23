import ChatTypes from 'keybase-bot/lib/types/chat1'
import * as Message from './message'
import search from './search'
import comment from './comment'
import reacji from './reacji'
import create from './create'
import {Context} from './context'

const reportError = (context: Context, channel: ChatTypes.ChatChannel, parsedMessage: Message.Message) =>
  context.bot.chat.send(channel, {
    body: parsedMessage.type === 'unknown' ? `Invalid command: ${parsedMessage.error}` : 'Unknown command',
  })

const reactAck = (context: Context, channel: ChatTypes.ChatChannel, id: number) => context.bot.chat.react(channel, id, ':eyes:')

const onMessage = (context: Context, kbMessage: ChatTypes.MsgSummary) => {
  try {
    // console.debug(kbMessage)
    const parsedMessage = Message.parseMessage(context, kbMessage)
    console.debug({msg: 'got message', parsedMessage})
    if (!parsedMessage) {
      // not a jirabot message
      return
    }
    switch (parsedMessage.type) {
      case Message.BotMessageType.Unknown:
        reportError(context, kbMessage.channel, parsedMessage)
        return
      case Message.BotMessageType.Search:
        reactAck(context, kbMessage.channel, kbMessage.id)
        search(context, kbMessage.channel, parsedMessage)
        return
      case Message.BotMessageType.Comment:
        reactAck(context, kbMessage.channel, kbMessage.id)
        comment(context, kbMessage.channel, parsedMessage)
        return
      case Message.BotMessageType.Reacji:
        reacji(context, kbMessage.channel, parsedMessage)
        return
      case Message.BotMessageType.Create:
        reactAck(context, kbMessage.channel, kbMessage.id)
        create(context, kbMessage.channel, parsedMessage)
        return
      default:
        console.error({error: 'how could this happen'})
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
    usage: `[issue-type] in <PROJECT> [for|assignee <kb-username>] "multi word summary" <description>`,
    title: 'Create a Jira ticket',
    body:
      'Examples:\n\n' +
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
    body: 'Examples:\n\n' + `!jira comment on TRIAGE-1024 this is already fixed\n`,
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
  context.bot.chat.watchAllChannelsForNewMessages(message => onMessage(context, message))
}
