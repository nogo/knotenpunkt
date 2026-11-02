package claim

import (
	"iter"
	"sort"
	"strings"
	"unicode"
)

// Match pairs a live claim with its count of distinct shared words.
type Match struct {
	Claim Claim
	Score int
}

// Recall returns positive word matches, best first, for a question or text.
// Exclude accepts claim ids, such as a proposal's supersedes and challenges.
// Ties use newer AsOf dates, then ascending ids. Empty input returns nothing.
func (l *nsLog) Recall(text string, limit int, exclude ...string) []Match {
	return recall(l.Live(), text, limit, exclude...)
}

// recall ranks claims against text; Recall and Store.Recall share it.
func recall(claims []Claim, text string, limit int, exclude ...string) []Match {
	if limit < 1 {
		return nil
	}
	query := words(text)
	if len(query) == 0 {
		return nil
	}
	omitted := make(map[string]bool, len(exclude))
	for _, id := range exclude {
		omitted[id] = true
	}
	var matches []Match
	seen := make(map[string]bool, len(query)) // reused: one map for every claim
	for _, c := range claims {
		if omitted[c.ID] {
			continue
		}
		clear(seen)
		score := 0
		for word := range wordSeq(c.Text) {
			if query[word] && !seen[word] {
				seen[word] = true
				score++
			}
		}
		if score > 0 {
			matches = append(matches, Match{Claim: c, Score: score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Claim.AsOf != b.Claim.AsOf {
			return a.Claim.AsOf > b.Claim.AsOf
		}
		return a.Claim.ID < b.Claim.ID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

// RecallClaim recalls candidates for a claim, excluding the claims it replaces
// or challenges. The input need not be appended to the log.
func (l *nsLog) RecallClaim(c Claim, limit int) []Match {
	exclude := append([]string(nil), c.Supersedes...)
	exclude = append(exclude, c.Challenges...)
	return l.Recall(c.Text, limit, exclude...)
}

// words normalizes Unicode words and removes common English and German words.
func words(text string) map[string]bool {
	out := map[string]bool{}
	for word := range wordSeq(text) {
		if !stopWords[word] {
			out[word] = true
		}
	}
	return out
}

// wordSeq yields the lowercased Unicode words of text, stop words included.
func wordSeq(text string) iter.Seq[string] {
	return strings.FieldsFuncSeq(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

var stopWords = func() map[string]bool {
	out := map[string]bool{}
	for _, word := range strings.Fields("a an the and or but is are was were be been to of in on for with as at by from this that it " +
		"der die das den dem des ein eine einer einem einen eines und oder aber ist sind war waren sein zu von im in am an auf für mit als bei aus dies diese dieser dieses es") {
		out[word] = true
	}
	return out
}()
