package claim

import (
	"unicode"
	"unicode/utf8"
)

// duplicateOf is the live claim or proposal in tree that says what c says
// about the same source: equal text, ignoring case and spacing, and the same
// source system and ref, or both without a source. A claim is no repeat of one
// it supersedes or retracts: a retraction or a change of trust keeps the text.
// Reach for it before a claim with a new id is written; an equal claim is a
// retry and never gets here. It allocates nothing, so proposing costs the same
// in a large namespace.
func duplicateOf(c Claim, tree []Claim) (Claim, bool) {
	for _, o := range tree {
		if o.ID == c.Retracted || containsID(c.Supersedes, o.ID) {
			continue
		}
		if sameSource(c.Source, o.Source) && sameText(c.Text, o.Text) {
			return o, true
		}
	}
	return Claim{}, false
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func sameSource(a, b *Source) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.System == b.System && a.Ref == b.Ref
}

// sameText compares two texts as if each were trimmed, every run of white
// space made one space, and its case folded. It walks both strings at once.
func sameText(a, b string) bool {
	x, y := folded{s: a}, folded{s: b}
	for {
		ra, rb := x.next(), y.next()
		if ra != rb {
			return false
		}
		if ra < 0 {
			return true
		}
	}
}

// folded reads a text as sameText sees it, one rune at a time.
type folded struct {
	s       string
	started bool // a rune other than white space was read
}

// next is the next rune: lowercase, one space for a run of white space between
// two words, or -1 at the end. White space at the start or the end yields
// nothing.
func (f *folded) next() rune {
	space := false
	for len(f.s) > 0 {
		r, n := utf8.DecodeRuneInString(f.s)
		if unicode.IsSpace(r) {
			f.s = f.s[n:]
			space = true
			continue
		}
		if space && f.started {
			return ' ' // the word's first rune is read next time
		}
		f.s = f.s[n:]
		f.started = true
		return unicode.ToLower(r)
	}
	return -1
}
