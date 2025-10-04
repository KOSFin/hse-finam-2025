package radar

import "strings"

func bilingual(en, ru string) string {
	en = strings.TrimSpace(en)
	ru = strings.TrimSpace(ru)
	if en == "" {
		return ru
	}
	if ru == "" {
		return en
	}
	return en + " / " + ru
}
