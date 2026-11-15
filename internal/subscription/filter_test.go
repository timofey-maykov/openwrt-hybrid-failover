package subscription

import (
	"reflect"
	"testing"
)

func TestFilterLinks(t *testing.T) {
	links := []Link{
		{URI: "vless://credential@ru.example:443", Name: "Финляндия"},
		{URI: "one", Name: "РОССИЯ Москва"},
		{URI: "two", Name: "US Premium"},
		{URI: "three", Name: "US free"},
		{URI: "four"},
	}
	for _, tc := range []struct {
		name  string
		terms []string
		want  []Link
	}{
		{"empty", []string{"", "  "}, links},
		{"case and phrases", []string{" россия ", "us FREE", "Россия"}, []Link{links[0], links[2], links[4]}},
		{"literal not regex", []string{".*"}, links},
		{"names only", []string{"credential", "ru.example"}, links},
		{"unnamed server stays", []string{"фин", "россия", "us"}, []Link{links[4]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, excluded := FilterLinks(links, tc.terms)
			if !reflect.DeepEqual(got, tc.want) || excluded != len(links)-len(tc.want) {
				t.Fatalf("got=%v excluded=%d want=%v", got, excluded, tc.want)
			}
		})
	}
}
