package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var opencodeStyle = styles.Register(chroma.MustNewStyle("opencode", chroma.StyleEntries{
	chroma.Text:                 "#eeeeee",
	chroma.Error:                "#e06c75",
	chroma.Comment:              "#808080",
	chroma.Keyword:              "#9d7cd8",
	chroma.Operator:             "#56b6c2",
	chroma.Punctuation:          "#eeeeee",
	chroma.Name:                 "#eeeeee",
	chroma.NameFunction:         "#fab283",
	chroma.NameBuiltin:          "#56b6c2",
	chroma.NameAttribute:        "#7fd88f",
	chroma.NameClass:            "#56b6c2",
	chroma.NameNamespace:        "#f5a742",
	chroma.NameConstant:         "#f5a742",
	chroma.NameDecorator:        "#9d7cd8",
	chroma.NameException:        "#e06c75",
	chroma.NameTag:              "#9d7cd8",
	chroma.NameVariable:         "#eeeeee",
	chroma.LiteralNumber:        "#f5a742",
	chroma.LiteralString:        "#7fd88f",
	chroma.LiteralStringEscape:  "#9d7cd8",
	chroma.GenericDeleted:       "#e06c75",
	chroma.GenericInserted:      "#7fd88f",
	chroma.GenericHeading:       "#fab283",
	chroma.GenericSubheading:    "#fab283",
	chroma.GenericPrompt:        "#808080",
	chroma.GenericOutput:        "#eeeeee",
}))

func highlightCode(lang, code string) string {
	if lang == "" {
		lang = "bash"
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil || iterator == nil {
		return code
	}

	var buf strings.Builder
	fmt := formatters.TTY16m
	err = fmt.Format(&buf, opencodeStyle, iterator)
	if err != nil {
		return code
	}
	return buf.String()
}

func renderHighlightedLines(lang, code string) []string {
	highlighted := highlightCode(lang, code)
	return strings.Split(strings.TrimRight(highlighted, "\n"), "\n")
}
