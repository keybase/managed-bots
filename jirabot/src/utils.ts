const quotes: {
  [key: string]: string
} = {
  '"': '"',
  "'": "'",
  '`': '`',
  '“': '”',
  '‘': '’',
}

const spaces = [' ', '\n', '\t']
const spacesRE = / |\n|\t/

// splits a string by white space, but respect quotes
export const split2 = (s: string) => {
  const chars = s.split('')
  const list = []
  let currentStr = ''
  for (let i = 0; i < chars.length; ) {
    if (quotes[chars[i]]) {
      // Try to find paired quote, and if not, continue with space based
      // splits.

      const activeQuoteStart = chars[i]
      let quotedStr = ''
      let foundQuoted = false
      for (let j = i + 1; j < chars.length; ++j) {
        const currentInQuoteChar = chars[j]
        if (currentInQuoteChar === quotes[activeQuoteStart]) {
          i = j + 1
          foundQuoted = true
          currentStr += quotedStr
          break
        } else {
          quotedStr = quotedStr + currentInQuoteChar
        }
      }
      if (foundQuoted) {
        // If found, continue with the next iteration so we do the i <
        // chars.length check, and also cover the case when next quote starts
        // immediately after previous one ends.
        continue
      }
    }

    if (spaces.includes(chars[i])) {
      list.push(currentStr)
      currentStr = ''
      for (; spaces.includes(chars[i]); ++i); // forward until non-space
      continue
    }

    currentStr += chars[i]
    ++i
  }

  if (currentStr) {
    list.push(currentStr)
  }

  return list
}

export const humanReadableArray = (list: Array<string>): string =>
  list.map(item => (item.match(spacesRE) ? `\`"${item}"\`` : `\`${item}\``)).join(' ')
