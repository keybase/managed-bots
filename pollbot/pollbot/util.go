package pollbot

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"
)

func formatTally(tally Tally, numChoices int) (res string) {
	res = "*Results*\n"
	if len(tally) == 0 {
		res += "_No votes yet_"
		return res
	}
	total := 0
	for _, t := range tally {
		total += t.votes
	}
	tallyMap := make(map[int]TallyResult)
	for _, t := range tally {
		tallyMap[t.choice] = t
	}
	for i := 0; i < numChoices; i++ {
		t, ok := tallyMap[i+1]
		if !ok {
			t.choice = i + 1
			t.votes = 0
		}
		s := ""
		if t.votes > 1 {
			s = "s"
		}
		prop := float64(t.votes / total)
		num := int(math.Max(10*prop, 1))
		bar := strings.Repeat("🟢", num)
		res += fmt.Sprintf("%s %s\n`(%.02f%%, %d vote%s)`\n\n", numberToEmoji(t.choice), bar, prop*100,
			t.votes, s)
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

func shortConvID(convID string) string {
	if len(convID) <= 20 {
		return convID
	}
	return convID[:20]
}

func encoder() *base64.Encoding {
	return base64.URLEncoding.WithPadding(base64.NoPadding)
}
