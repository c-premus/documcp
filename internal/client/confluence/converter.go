package confluence

import (
	"regexp"
	"strconv"
	"strings"
)

// Precompiled regexes for storageToMarkdown and helper functions.
var (
	// Heading regexes indexed by level (1-6). Index 0 is unused.
	reHeadingOpen [7]*regexp.Regexp

	// Inline/block element regexes.
	reBr      = regexp.MustCompile(`<br\s*/?>`)
	reHr      = regexp.MustCompile(`<hr\s*/?>`)
	rePOpen   = regexp.MustCompile(`<p[^>]*>`)
	rePreOpen = regexp.MustCompile(`<pre[^>]*>`)
	reHTMLTag = regexp.MustCompile(`<[^>]+>`)

	// Tag open regexes keyed by tag name (with \b to prevent partial matches).
	reTagOpen = map[string]*regexp.Regexp{
		"strong":     regexp.MustCompile(`<strong[^>]*>`),
		"b":          regexp.MustCompile(`<b\b[^>]*>`),
		"em":         regexp.MustCompile(`<em[^>]*>`),
		"i":          regexp.MustCompile(`<i\b[^>]*>`),
		"code":       regexp.MustCompile(`<code[^>]*>`),
		"blockquote": regexp.MustCompile(`<blockquote[^>]*>`),
	}

	// ConvertCodeMacros regexes.
	reCodeMacro = regexp.MustCompile(
		`(?s)<ac:structured-macro\s[^>]*ac:name="code"[^>]*>` +
			`(.*?)` +
			`</ac:structured-macro>`,
	)
	reCodeLang = regexp.MustCompile(
		`<ac:parameter\s+ac:name="language"[^>]*>([^<]+)</ac:parameter>`,
	)
	reCDATA = regexp.MustCompile(`(?s)<!\[CDATA\[(.*?)\]\]>`)
	rePTB   = regexp.MustCompile(
		`(?s)<ac:plain-text-body[^>]*>(.*?)</ac:plain-text-body>`,
	)

	// ConvertPanelMacros regexes (keyed by panel name).
	rePanelMacro = map[string]*regexp.Regexp{
		"info":    regexp.MustCompile(`(?s)<ac:structured-macro\s[^>]*ac:name="info"[^>]*>(.*?)</ac:structured-macro>`),
		"note":    regexp.MustCompile(`(?s)<ac:structured-macro\s[^>]*ac:name="note"[^>]*>(.*?)</ac:structured-macro>`),
		"warning": regexp.MustCompile(`(?s)<ac:structured-macro\s[^>]*ac:name="warning"[^>]*>(.*?)</ac:structured-macro>`),
		"tip":     regexp.MustCompile(`(?s)<ac:structured-macro\s[^>]*ac:name="tip"[^>]*>(.*?)</ac:structured-macro>`),
	}
	reRichTextBody = regexp.MustCompile(
		`(?s)<ac:rich-text-body[^>]*>(.*?)</ac:rich-text-body>`,
	)

	// StripACMacros regex.
	reACMacro = regexp.MustCompile(`</?ac:[^>]+>`)

	// ConvertLinks regexes.
	reLinkTag = regexp.MustCompile(`(?s)<a\s[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)

	// ConvertImages regexes.
	reImgTag  = regexp.MustCompile(`<img\s[^>]*src="([^"]*)"[^>]*/?>`)
	reAltAttr = regexp.MustCompile(`alt="([^"]*)"`)

	// ConvertTables regexes.
	reTable = regexp.MustCompile(`(?s)<table[^>]*>(.*?)</table>`)
	reRow   = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	reCell  = regexp.MustCompile(`(?s)<(?:th|td)[^>]*>(.*?)</(?:th|td)>`)

	// ConvertLists regexes.
	reOL = regexp.MustCompile(`(?s)<ol[^>]*>(.*?)</ol>`)
	reUL = regexp.MustCompile(`(?s)<ul[^>]*>(.*?)</ul>`)

	// ConvertListItems regexes.
	reLI = regexp.MustCompile(`(?s)<li[^>]*>(.*?)</li>`)

	// CleanWhitespace regex.
	reMultiNL = regexp.MustCompile(`\n{3,}`)
)

func init() {
	for i := 1; i <= 6; i++ {
		tag := strconv.Itoa(i)
		reHeadingOpen[i] = regexp.MustCompile(`<h` + tag + `\b[^>]*>`)
	}
}

// storageToMarkdown converts Confluence storage format XHTML to Markdown.
// It handles the common element set used in Confluence pages. Unknown tags
// and Confluence macros are stripped, preserving their text content where
// possible.
func storageToMarkdown(html string) string {
	if html == "" {
		return ""
	}

	s := html

	// Normalize line endings.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// --- Confluence macros ---

	// Code blocks: <ac:structured-macro ac:name="code">...<ac:plain-text-body><![CDATA[...]]></ac:plain-text-body>...</ac:structured-macro>
	s = convertCodeMacros(s)

	// Info/note/warning/tip panels.
	s = convertPanelMacros(s)

	// Strip remaining ac: macros but keep their body text.
	s = stripACMacros(s)

	// --- Standard HTML elements ---

	// Headings.
	for i := 6; i >= 1; i-- {
		prefix := strings.Repeat("#", i)
		tag := strconv.Itoa(i)
		s = reHeadingOpen[i].ReplaceAllString(s, "\n\n"+prefix+" ")
		s = strings.ReplaceAll(s, "</h"+tag+">", "\n\n")
	}

	// Bold / strong.
	s = replaceTag(s, "strong", "**", "**")
	s = replaceTag(s, "b", "**", "**")

	// Italic / emphasis.
	s = replaceTag(s, "em", "*", "*")
	s = replaceTag(s, "i", "*", "*")

	// Inline code.
	s = replaceTag(s, "code", "`", "`")

	// Links: <a href="url">text</a>
	s = convertLinks(s)

	// Images: <img src="url" alt="text" /> or <ac:image>
	s = convertImages(s)

	// Line breaks.
	s = reBr.ReplaceAllString(s, "\n")

	// Horizontal rules.
	s = reHr.ReplaceAllString(s, "\n\n---\n\n")

	// Tables.
	s = convertTables(s)

	// Lists.
	s = convertLists(s)

	// Paragraphs.
	s = rePOpen.ReplaceAllString(s, "\n\n")
	s = strings.ReplaceAll(s, "</p>", "\n\n")

	// Blockquotes.
	s = replaceTag(s, "blockquote", "\n\n> ", "\n\n")

	// Preformatted text.
	s = rePreOpen.ReplaceAllString(s, "\n\n```\n")
	s = strings.ReplaceAll(s, "</pre>", "\n```\n\n")

	// Strip any remaining HTML tags.
	s = reHTMLTag.ReplaceAllString(s, "")

	// Decode common HTML entities.
	s = decodeEntities(s)

	// Clean up excessive whitespace.
	s = cleanWhitespace(s)

	return strings.TrimSpace(s)
}

// convertCodeMacros extracts code blocks from Confluence ac:structured-macro
// elements with ac:name="code".
func convertCodeMacros(s string) string {
	return reCodeMacro.ReplaceAllStringFunc(s, func(match string) string {
		// Try to extract the language from ac:parameter ac:name="language".
		lang := ""
		if m := reCodeLang.FindStringSubmatch(match); len(m) > 1 {
			lang = strings.TrimSpace(m[1])
		}

		// Extract body from CDATA.
		body := ""
		if m := reCDATA.FindStringSubmatch(match); len(m) > 1 {
			body = m[1]
		} else {
			// Fallback: extract from plain-text-body tags.
			if m := rePTB.FindStringSubmatch(match); len(m) > 1 {
				body = m[1]
			}
		}

		return "\n\n```" + lang + "\n" + strings.TrimSpace(body) + "\n```\n\n"
	})
}

// convertPanelMacros converts info, note, warning, and tip macros to blockquotes.
func convertPanelMacros(s string) string {
	panels := []struct {
		name   string
		prefix string
	}{
		{"info", "INFO"},
		{"note", "NOTE"},
		{"warning", "WARNING"},
		{"tip", "TIP"},
	}

	for _, p := range panels {
		re := rePanelMacro[p.name]
		prefix := p.prefix
		s = re.ReplaceAllStringFunc(s, func(match string) string {
			// Extract body from ac:rich-text-body.
			body := ""
			if m := reRichTextBody.FindStringSubmatch(match); len(m) > 1 {
				body = strings.TrimSpace(m[1])
			}
			// Strip inner HTML tags for a clean blockquote.
			body = reHTMLTag.ReplaceAllString(body, "")
			body = strings.TrimSpace(body)
			return "\n\n> **" + prefix + ":** " + body + "\n\n"
		})
	}

	return s
}

// stripACMacros removes any remaining ac: namespaced elements, preserving
// text content inside them.
func stripACMacros(s string) string {
	return reACMacro.ReplaceAllString(s, "")
}

// replaceTag replaces simple open/close HTML tags with prefix/suffix strings.
func replaceTag(s, tag, prefix, suffix string) string {
	if re, ok := reTagOpen[tag]; ok {
		s = re.ReplaceAllString(s, prefix)
	} else {
		re := regexp.MustCompile(`<` + tag + `\b[^>]*>`)
		s = re.ReplaceAllString(s, prefix)
	}
	s = strings.ReplaceAll(s, "</"+tag+">", suffix)
	return s
}

// convertLinks converts <a href="...">text</a> to [text](url).
func convertLinks(s string) string {
	return reLinkTag.ReplaceAllStringFunc(s, func(match string) string {
		m := reLinkTag.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		href := m[1]
		text := strings.TrimSpace(m[2])
		// Strip any nested tags from link text.
		text = reHTMLTag.ReplaceAllString(text, "")
		if text == "" {
			text = href
		}
		return "[" + text + "](" + href + ")"
	})
}

// convertImages converts <img> tags to ![alt](src).
func convertImages(s string) string {
	return reImgTag.ReplaceAllStringFunc(s, func(match string) string {
		srcM := reImgTag.FindStringSubmatch(match)
		if len(srcM) < 2 {
			return ""
		}
		src := srcM[1]
		alt := ""
		if m := reAltAttr.FindStringSubmatch(match); len(m) > 1 {
			alt = m[1]
		}
		return "![" + alt + "](" + src + ")"
	})
}

// convertTables converts HTML tables to Markdown tables.
func convertTables(s string) string {
	return reTable.ReplaceAllStringFunc(s, func(match string) string {
		var b strings.Builder

		// Extract rows.
		rows := reRow.FindAllStringSubmatch(match, -1)

		isFirstRow := true

		for _, row := range rows {
			if len(row) < 2 {
				continue
			}
			content := row[1]

			// Extract cells (th or td).
			cells := reCell.FindAllStringSubmatch(content, -1)

			cellTexts := make([]string, 0, len(cells))
			for _, cell := range cells {
				if len(cell) < 2 {
					cellTexts = append(cellTexts, "")
					continue
				}
				text := reHTMLTag.ReplaceAllString(cell[1], "")
				text = strings.TrimSpace(text)
				cellTexts = append(cellTexts, text)
			}

			b.WriteString("| ")
			b.WriteString(strings.Join(cellTexts, " | "))
			b.WriteString(" |\n")

			// Add separator after header row.
			if isFirstRow {
				b.WriteString("|")
				for range cellTexts {
					b.WriteString(" --- |")
				}
				b.WriteString("\n")
				isFirstRow = false
			}
		}

		return "\n\n" + b.String() + "\n"
	})
}

// convertLists converts <ul>, <ol>, and <li> elements to Markdown lists.
func convertLists(s string) string {
	// Process ordered lists.
	s = reOL.ReplaceAllStringFunc(s, func(match string) string {
		m := reOL.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		return "\n\n" + convertListItems(m[1], true) + "\n"
	})

	// Process unordered lists.
	s = reUL.ReplaceAllStringFunc(s, func(match string) string {
		m := reUL.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		return "\n\n" + convertListItems(m[1], false) + "\n"
	})

	return s
}

// convertListItems extracts <li> elements and formats them as Markdown list items.
func convertListItems(s string, ordered bool) string {
	items := reLI.FindAllStringSubmatch(s, -1)

	var b strings.Builder
	for idx, item := range items {
		if len(item) < 2 {
			continue
		}
		text := reHTMLTag.ReplaceAllString(item[1], "")
		text = strings.TrimSpace(text)
		if ordered {
			b.WriteString(strconv.Itoa(idx+1) + ". " + text)
		} else {
			b.WriteString("- " + text)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// decodeEntities replaces common HTML entities with their text equivalents.
func decodeEntities(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
		"&ndash;", "-",
		"&mdash;", "--",
		"&laquo;", "<<",
		"&raquo;", ">>",
		"&copy;", "(c)",
		"&reg;", "(R)",
		"&trade;", "(TM)",
	)
	return replacer.Replace(s)
}

// cleanWhitespace collapses excessive blank lines and trims trailing spaces.
func cleanWhitespace(s string) string {
	// Collapse 3+ newlines into 2.
	s = reMultiNL.ReplaceAllString(s, "\n\n")

	// Remove trailing whitespace on each line.
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}
