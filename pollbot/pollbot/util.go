package pollbot

import "fmt"

func formatTally(tally Tally) (res string) {
	res = "*Results*\n"
	if len(tally) == 0 {
		res += "_No votes yet_"
		return res
	}
	for _, t := range tally {
		s := ""
		if t.votes > 1 {
			s = "s"
		}
		res += fmt.Sprintf("%s  `%d vote%s`\n", numberToEmoji(t.choice), t.votes, s)
	}
	return res
}

func numberToEmoji(v int) string {
	switch v {
	case 1:
		return ":one:"
	case 2:
		return ":two:"
	case 3:
		return ":three:"
	case 4:
		return ":four:"
	case 5:
		return ":five:"
	case 6:
		return ":six:"
	case 7:
		return ":seven:"
	case 8:
		return ":eight:"
	case 9:
		return ":nine:"
	case 10:
		return ":ten:"
	default:
		return fmt.Sprintf("%d", v)
	}
}
