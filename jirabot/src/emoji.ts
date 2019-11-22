export const emojiToNum = (num: string): null | number => {
  switch (num) {
    case ':zero:':
      return 0
    case ':one:':
      return 1
    case ':two:':
      return 2
    case ':three:':
      return 3
    case ':four:':
      return 4
    case ':five:':
      return 5
    case ':six:':
      return 6
    case ':seven:':
      return 7
    case ':eight:':
      return 8
    case ':nine:':
      return 9
    case ':keycap_ten:':
      return 10
    default:
      return null
  }
}

export const numToEmoji = (num: number): string => {
  switch (num) {
    case 0:
      return ':zero:'
    case 1:
      return ':one:'
    case 2:
      return ':two:'
    case 3:
      return ':three:'
    case 4:
      return ':four:'
    case 5:
      return ':five:'
    case 6:
      return ':six:'
    case 7:
      return ':seven:'
    case 8:
      return ':eight:'
    case 9:
      return ':nine:'
    case 10:
      return ':keycap_ten:'
    default:
      return ':question:'
  }
}

export const statusToEmoji = (status: string) => {
  switch (status) {
    case 'Done':
      return ':white_check_mark:'
    case 'To Do':
      return ':statue_of_liberty:'
    default:
      return ':building_construction:'
  }
}
