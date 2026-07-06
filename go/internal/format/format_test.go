package format

import "testing"

func TestInt_LocaleAware(t *testing.T) {
	cases := []struct {
		loc  Locale
		in   int
		want string
	}{
		{EN, 1234567, "1,234,567"},
		{CS, 1234567, "1 234 567"},
		{DE, 1234567, "1.234.567"},
		{FR, -42, "-42"},
		{EN, 0, "0"},
		{EN, 999, "999"},
		{EN, -12345, "-12,345"},
	}
	for _, c := range cases {
		if got := Int(c.loc, c.in); got != c.want {
			t.Errorf("Int(%v, %d) = %q, want %q", c.loc, c.in, got, c.want)
		}
	}
}

func TestFromAcceptLanguage(t *testing.T) {
	cases := map[string]Locale{
		"":                        EN,
		"en-US,en;q=0.9":          EN,
		"cs-CZ,cs;q=0.9,en;q=0.5": CS,
		"de;q=0.9,en;q=0.5":       DE,
		"sk,en":                   SK,
		"xx-XX":                   EN, // unknown
		"fr-FR,fr;q=0.9":          FR,
	}
	for header, want := range cases {
		if got := FromAcceptLanguage(header); got != want {
			t.Errorf("FromAcceptLanguage(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestDuration(t *testing.T) {
	cases := map[int]string{
		0:    "-",
		-5:   "-",
		7:    "0:07",
		60:   "1:00",
		125:  "2:05",
		3600: "1:00:00",
		3725: "1:02:05",
	}
	for in, want := range cases {
		if got := Duration(in); got != want {
			t.Errorf("Duration(%d) = %q, want %q", in, got, want)
		}
	}
}
