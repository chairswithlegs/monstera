package service

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/microcosm-cc/bluemonday"
	"mvdan.cc/xurls/v2"
)

// MentionRef is a resolved @mention for persistence.
type MentionRef struct {
	Username  string
	Domain    *string
	AccountID string
}

// MentionResolver resolves a mention (username, optional domain) to an account.
type MentionResolver func(username string, domain *string) *domain.Account

// RenderResult is the output of the content rendering pipeline.
type RenderResult struct {
	HTML     string
	Mentions []MentionRef
	Tags     []string
}

var (
	mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_]+)(?:@([a-zA-Z0-9.\-]+))?`)
	hashtagRegex = regexp.MustCompile(`#([a-zA-Z0-9_]+)`)
	strictURLs   = xurls.Strict()
)

// Render transforms plain text to HTML with mentions and hashtags extracted.
func Render(text string, instanceDomain string, resolve MentionResolver) (RenderResult, error) {
	strict := bluemonday.StrictPolicy()
	text = strict.Sanitize(text)

	var mentions []MentionRef
	text = replaceMentions(text, resolve, &mentions)
	text, tags := replaceHashtags(text, instanceDomain)
	text = replaceURLs(text)
	text = paragraphWrap(text)

	ugc := bluemonday.UGCPolicy()
	ugc.AllowElements("p", "br", "a", "span")
	ugc.AllowAttrs("href", "rel", "class", "target").OnElements("a")
	ugc.AllowAttrs("class").OnElements("span")
	text = ugc.Sanitize(text)

	return RenderResult{HTML: text, Mentions: mentions, Tags: tags}, nil
}

func replaceMentions(text string, resolve MentionResolver, out *[]MentionRef) string {
	matches := mentionRegex.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	type repl struct {
		start, end int
		repl       string
	}
	var replacements []repl
	for _, m := range matches {
		username := text[m[2]:m[3]]
		var domain *string
		if m[4] != -1 && m[5] != -1 {
			d := text[m[4]:m[5]]
			domain = &d
		}
		account := resolve(username, domain)
		if account == nil {
			continue
		}
		*out = append(*out, MentionRef{Username: username, Domain: domain, AccountID: account.ID})
		replacement := `<span class="h-card"><a href="` + account.APID + `" class="u-url mention">@<span>` + username + `</span></a></span>`
		replacements = append(replacements, repl{m[0], m[1], replacement})
	}
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		text = text[:r.start] + r.repl + text[r.end:]
	}
	return text
}

func replaceHashtags(text string, instanceDomain string) (string, []string) {
	var tags []string
	seen := make(map[string]bool)
	text = hashtagRegex.ReplaceAllStringFunc(text, func(match string) string {
		name := strings.ToLower(match[1:])
		if !seen[name] {
			seen[name] = true
			tags = append(tags, name)
		}
		url := "https://" + instanceDomain + "/tags/" + name
		return `<a href="` + url + `" class="mention hashtag" rel="tag">#<span>` + name + `</span></a>`
	})
	return text, tags
}

func replaceURLs(text string) string {
	return strictURLs.ReplaceAllStringFunc(text, func(match string) string {
		display := match
		if strings.HasPrefix(display, "https://") {
			display = display[8:]
		} else if strings.HasPrefix(display, "http://") {
			display = display[7:]
		}
		if utf8.RuneCountInString(display) > 30 {
			runes := []rune(display)
			display = string(runes[:30]) + "…"
		}
		return `<a href="` + match + `" rel="nofollow noopener noreferrer" target="_blank">` + display + `</a>`
	})
}

// CountStatusCharacters returns the effective character count for status length validation.
// Each URL counts as 23 characters (Mastodon convention); other runes count as 1.
func CountStatusCharacters(text string) int {
	urls := strictURLs.FindAllString(text, -1)
	totalRunes := utf8.RuneCountInString(text)
	urlRunes := 0
	for _, u := range urls {
		urlRunes += utf8.RuneCountInString(u)
	}
	return totalRunes - urlRunes + 23*len(urls)
}

func paragraphWrap(text string) string {
	paragraphs := strings.Split(text, "\n\n")
	for i, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			paragraphs[i] = ""
			continue
		}
		paragraphs[i] = "<p>" + strings.ReplaceAll(p, "\n", "<br>") + "</p>"
	}
	return strings.Join(paragraphs, "")
}
