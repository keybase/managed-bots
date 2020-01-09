package pollbot

import (
	"fmt"
	"math"
	"strings"

	"github.com/keybase/managed-bots/base"
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
		if t.votes != 1 {
			s = "s"
		}
		prop := float64(t.votes) / float64(total)
		num := int(math.Max(10*prop, 1))
		bar := strings.Repeat("🟢", num)
		res += fmt.Sprintf("%s %s\n`(%.02f%%, %d vote%s)`\n\n", base.NumberToEmoji(t.choice), bar, prop*100,
			t.votes, s)
	}
	return res
}
