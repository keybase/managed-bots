import ChatTypes from 'keybase-bot/lib/types/chat1'
import {ReacjiMessage} from './message'
import {Context} from './context'

export default (_: Context, __: ChatTypes.ChatChannel, ___: ReacjiMessage) => Promise.reject('not used')
