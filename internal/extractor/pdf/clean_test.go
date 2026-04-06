package pdf

import "testing"

func TestCleanText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normalizes CRLF to LF",
			input: "Line one\r\nLine two\r\n",
			want:  "Line one\nLine two\n",
		},
		{
			name:  "normalizes bare CR to LF",
			input: "Line one\rLine two\r",
			want:  "Line one\nLine two\n",
		},
		{
			name:  "rejoins broken bracket references",
			input: "some text [\n1\n] more text",
			want:  "some text [1] more text",
		},
		{
			name:  "rejoins alphabetic bracket references",
			input: "see [\na\n] and [\nb\n]",
			want:  "see [a] and [b]",
		},
		{
			name:  "rejoins multi-digit bracket references",
			input: "ref [\n42\n] here",
			want:  "ref [42] here",
		},
		{
			name:  "collapses excessive blank lines",
			input: "Para one\n\n\n\n\nPara two",
			want:  "Para one\n\n\nPara two",
		},
		{
			name:  "preserves double blank lines",
			input: "Para one\n\n\nPara two",
			want:  "Para one\n\n\nPara two",
		},
		{
			name:  "handles combined issues",
			input: "Text [\r\n1\r\n] more\r\n\r\n\r\n\r\n\r\nNext",
			want:  "Text [1] more\n\n\nNext",
		},
		{
			name:  "returns empty string unchanged",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := cleanText(tt.input)
			if got != tt.want {
				t.Errorf("cleanText() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestJoinContinuationLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "joins lowercase continuation with space",
			input: "This is an \nadvice page\n from \nWikiProject",
			want:  "This is an advice page from \nWikiProject",
		},
		{
			name:  "inserts space when missing between words",
			input: "writing\nstyle",
			want:  "writing style",
		},
		{
			name:  "joins punctuation continuation",
			input: "Wikipedia\n:\nSigns of AI",
			want:  "Wikipedia:\nSigns of AI",
		},
		{
			name:  "joins closing bracket continuation",
			input: "Some text\n) more text",
			want:  "Some text) more text",
		},
		{
			name:  "preserves uppercase line starts",
			input: "First sentence.\nSecond sentence.",
			want:  "First sentence.\nSecond sentence.",
		},
		{
			name:  "preserves blank lines as paragraph breaks",
			input: "Paragraph one.\n\nParagraph two.",
			want:  "Paragraph one.\n\nParagraph two.",
		},
		{
			name:  "joins whitespace-leading continuation",
			input: "Some text\n that continues",
			want:  "Some text that continues",
		},
		{
			name:  "joins comma continuation",
			input: "Wikipedia policy\n, as it has not been",
			want:  "Wikipedia policy, as it has not been",
		},
		{
			name:  "joins period continuation",
			input: "WikiProject AI Cleanup\n.",
			want:  "WikiProject AI Cleanup.",
		},
		{
			name:  "handles realistic PDF fragment",
			input: "This is an \nadvice page\n from \nWikiProject AI Cleanup\n.\nThis page is not a \nWikipedia policy\n, as it has not been \nreviewed by the community\n.",
			want:  "This is an advice page from \nWikiProject AI Cleanup.\nThis page is not a \nWikipedia policy, as it has not been reviewed by the community.",
		},
		{
			name:  "no double space when previous ends with space",
			input: "some text \ncontinues here",
			want:  "some text continues here",
		},
		{
			name:  "no space before period",
			input: "WikiProject AI Cleanup\n.",
			want:  "WikiProject AI Cleanup.",
		},
		{
			name:  "single line unchanged",
			input: "Just one line",
			want:  "Just one line",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "joins em dash continuation",
			input: "Some text\n\u2014more text",
			want:  "Some text\u2014more text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := joinContinuationLines(tt.input)
			if got != tt.want {
				t.Errorf("joinContinuationLines() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestIsAttaching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"period attaches", '.', true},
		{"comma attaches", ',', true},
		{"close paren attaches", ')', true},
		{"em dash attaches", '\u2014', true},
		{"right curly quote attaches", '\u2019', true},
		{"space attaches", ' ', true},
		{"lowercase does not attach", 's', false},
		{"left curly quote does not attach", '\u201C', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isAttaching(tt.r)
			if got != tt.want {
				t.Errorf("isAttaching(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestIsContinuation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"lowercase a", 'a', true},
		{"lowercase z", 'z', true},
		{"uppercase A", 'A', false},
		{"digit", '1', false},
		{"space", ' ', true},
		{"tab", '\t', true},
		{"comma", ',', true},
		{"period", '.', true},
		{"semicolon", ';', true},
		{"colon", ':', true},
		{"close paren", ')', true},
		{"close bracket", ']', true},
		{"open paren", '(', false},
		{"open bracket", '[', false},
		{"em dash", '\u2014', true},
		{"en dash", '\u2013', true},
		{"left curly quote", '\u201C', true},
		{"right curly quote", '\u201D', true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isContinuation(tt.r)
			if got != tt.want {
				t.Errorf("isContinuation(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}
