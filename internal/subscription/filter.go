package subscription

import "strings"

// FilterLinks excludes servers whose display name contains any keyword.
// Keywords are literal substrings, matched without case; credentials and
// endpoints in the URI are deliberately not searched.
func FilterLinks(links []Link, keywords []string) ([]Link, int) {
	var terms []string
	for _, word := range keywords {
		if word = strings.ToLower(strings.TrimSpace(word)); word != "" {
			terms = append(terms, word)
		}
	}
	if len(terms) == 0 {
		return links, 0
	}
	kept := make([]Link, 0, len(links))
	for _, link := range links {
		name := strings.ToLower(link.Name)
		excluded := false
		for _, term := range terms {
			if strings.Contains(name, term) {
				excluded = true
				break
			}
		}
		if !excluded {
			kept = append(kept, link)
		}
	}
	return kept, len(links) - len(kept)
}
