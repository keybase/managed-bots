package elastiwatch

import (
	"cmp"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type entry struct {
	Message       string `json:"message"`
	Msg           string `json:"msg"`
	Severity      string `json:"severity"`
	Level         string `json:"level"`
	UID           string `json:"uid"`
	Time          string `json:"time"`
	Timestamp     string `json:"@timestamp"`
	Program       string `json:"program"`
	SyslogProgram string `json:"syslog_program"`
	Instance      string `json:"instance"`
	Section       string `json:"section"`
	Error         any    `json:"error"`
	parts         []string
}

func normalizeSeverity(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "WARN", "WARNING":
		return "WARNING"
	case "ERR", "ERROR":
		return "ERROR"
	case "FATAL", "PANIC", "CRIT", "CRITICAL":
		return "CRITICAL"
	case "INFO", "INFORMATION", "NOTICE":
		return "INFO"
	case "DEBUG", "TRACE":
		return "DEBUG"
	default:
		return strings.ToUpper(strings.TrimSpace(s))
	}
}

func stringifyField(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func newEntry(dat []byte) (*entry, error) {
	entry := new(entry)
	if err := json.Unmarshal(dat, entry); err != nil {
		return nil, err
	}
	entry.Message = cmp.Or(entry.Message, entry.Msg)
	entry.Severity = normalizeSeverity(cmp.Or(entry.Severity, entry.Level))
	entry.Time = cmp.Or(entry.Time, entry.Timestamp)
	entry.Program = cmp.Or(entry.Program, entry.SyslogProgram, entry.Instance, entry.Section)
	if errText := stringifyField(entry.Error); errText != "" && !strings.Contains(entry.Message, errText) {
		entry.Message = strings.TrimSpace(entry.Message + " " + errText)
	}
	for _, s := range []string{entry.Program, entry.UID} {
		if s != "" {
			entry.parts = append(entry.parts, s)
		}
	}
	entry.parts = append(entry.parts, strings.Fields(entry.Message)...)
	return entry, nil
}

func (e *entry) words() []string {
	return e.parts
}

func (e *entry) numWords() int {
	return len(e.parts)
}

func (e *entry) String() string {
	return fmt.Sprintf("%s %s %s %s %s", e.Time, e.Program, e.Severity, e.UID, e.Message)
}

type chunk struct {
	Severity string
	Count    int
	Message  string
	Items    []string
	Time     string
}

type grouper interface {
	Group([]*entry) []chunk
}

type tree struct {
	entry    *entry
	count    int
	subtrees []*tree
	words    []string
	time     time.Time
}

func newTree(e *entry) *tree {
	etime, err := time.Parse(time.RFC3339, e.Time)
	if err != nil {
		etime = time.Time{}
	}
	return &tree{
		entry: e,
		count: 1,
		words: e.words(),
		time:  etime,
	}
}

func (t *tree) toChunk() chunk {
	c := chunk{}
	c.Severity = t.entry.Severity
	c.Count = t.count
	c.Message = strings.Join(t.words, " ")
	c.Time = t.time.Format("3:04PM")
	for _, t := range t.subtrees {
		c.Items = append(c.Items, t.entry.String())
	}
	return c
}

type treeifyGrouper struct {
	maxGroups int
}

var _ grouper = (*treeifyGrouper)(nil)

func newTreeifyGrouper(maxGroups int) *treeifyGrouper {
	return &treeifyGrouper{
		maxGroups: maxGroups,
	}
}

func (t *treeifyGrouper) key(e *entry) string {
	return fmt.Sprintf("%s %d", e.Severity, e.numWords())
}

func (t *treeifyGrouper) commonAncestor(aparts, bparts []string) (res []string) {
	for index, ap := range aparts {
		if ap != bparts[index] {
			res = append(res, "___")
		} else {
			res = append(res, ap)
		}
	}
	return res
}

func (t *treeifyGrouper) errDist(aparts, bparts []string) (res int) {
	for index, ap := range aparts {
		if ap != bparts[index] {
			res++
		}
	}
	return res
}

func (t *treeifyGrouper) buildForest(trees []*tree) (forest []*tree) {
	if len(trees) == 0 {
		return trees
	}
	numParts := len(trees[0].words)
	minDist := min(numParts, t.maxGroups)
	for _, loneTree := range trees {
		consumed := false
		for _, forestTree := range forest {
			if t.errDist(forestTree.words, loneTree.words) <= minDist {
				forestTree.subtrees = append(forestTree.subtrees, loneTree)
				forestTree.count++
				forestTree.words = t.commonAncestor(forestTree.words, loneTree.words)
				if loneTree.time.After(forestTree.time) {
					forestTree.time = loneTree.time
				}
				consumed = true
				break
			}
		}
		if !consumed {
			loneTree.subtrees = append(loneTree.subtrees, loneTree)
			forest = append(forest, loneTree)
		}
	}
	return forest
}

func (t *treeifyGrouper) treeify(entries []*entry) (res []*tree) {
	groups := make(map[string][]*tree)
	for _, e := range entries {
		key := t.key(e)
		tree := newTree(e)
		groups[key] = append(groups[key], tree)
	}
	for _, group := range groups {
		res = append(res, t.buildForest(group)...)
	}
	return res
}

func severityRank(severity string) int {
	switch severity {
	case "DEBUG":
		return 0
	case "INFO":
		return 1
	case "WARNING":
		return 2
	case "ERROR":
		return 3
	case "CRITICAL":
		return 4
	default:
		return 0
	}
}

type byValue []chunk

func (b byValue) Len() int      { return len(b) }
func (b byValue) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byValue) Less(i, j int) bool {
	irank := severityRank(b[i].Severity)
	jrank := severityRank(b[j].Severity)
	if irank > jrank {
		return true
	}
	if irank < jrank {
		return false
	}
	return len(b[i].Items) > len(b[j].Items)
}

func (t *treeifyGrouper) Group(entries []*entry) (res []chunk) {
	trees := t.treeify(entries)
	for _, tree := range trees {
		res = append(res, tree.toChunk())
	}
	sort.Sort(byValue(res))
	return res
}
