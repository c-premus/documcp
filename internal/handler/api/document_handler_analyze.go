package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/c-premus/documcp/internal/service"
)

// analyzeResponse is the JSON representation of a document analysis result.
type analyzeResponse struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	WordCount   int      `json:"word_count"`
	ReadingTime int      `json:"reading_time"`
	Language    string   `json:"language"`
}

// Analyze handles POST /api/documents/analyze — extract and analyze an uploaded file.
func (h *DocumentHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "file is required")
		return
	}
	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	mimeType, ok := service.AllowedMIMETypes[ext]
	if !ok {
		errorResponse(w, http.StatusBadRequest, fmt.Sprintf("unsupported file type: %q", ext))
		return
	}

	// Write to a temp file for extraction. Use the configured worker temp
	// dir (under STORAGE_BASE_PATH) so operators can size the scratch area
	// with a dedicated volume or tmpfs.
	tmpFile, err := os.CreateTemp(h.workerTempDir, "analyze-*"+ext)
	if err != nil {
		h.logger.Error("creating temp file for analysis", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to process file")
		return
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name()) //nolint:gosec // tmpFile created by os.CreateTemp in this function
	}()

	if _, err = io.Copy(tmpFile, file); err != nil {
		h.logger.Error("writing temp file for analysis", "error", err)
		errorResponse(w, http.StatusInternalServerError, "failed to process file")
		return
	}
	_ = tmpFile.Close()

	ext2, err := h.pipeline.ExtractorRegistry().ForMIMEType(mimeType)
	if err != nil {
		errorResponse(w, http.StatusUnprocessableEntity, "no extractor for type: "+mimeType)
		return
	}

	result, err := ext2.Extract(r.Context(), tmpFile.Name())
	if err != nil {
		h.logger.Error("extracting content for analysis", "error", err)
		errorResponse(w, http.StatusInternalServerError, "content extraction failed")
		return
	}

	content := result.Content
	wordCount := len(strings.Fields(content))
	readingTime := max(wordCount/200, 1)

	// Derive title: extractor metadata > first H1 > filename.
	title := metadataString(result.Metadata, "title", "Title")
	if title == "" {
		title = firstHeading(content)
	}
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	// Derive description: extractor metadata > first non-heading paragraph.
	description := metadataString(result.Metadata, "description")
	if description == "" {
		description = firstParagraph(content)
	}

	// Derive tags: metadata subjects first, then headings, then keyword frequency.
	tags := metadataSubjects(result.Metadata)
	if len(tags) < 3 {
		tags = extractHeadingTags(content)
	}
	if len(tags) < 3 {
		tags = extractKeywords(content)
	}

	resp := analyzeResponse{
		Title:       title,
		Description: description,
		Tags:        tags,
		WordCount:   wordCount,
		ReadingTime: readingTime,
		Language:    detectLanguage(content),
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"data": resp,
	})
}

// metadataString returns the first non-empty string value found under the given
// keys in the extractor metadata map. Keys are tried in order (e.g. "title",
// "Title") to handle format-specific casing differences.
func metadataString(metadata map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := metadata[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// firstHeading returns the text of the first ATX heading (# Title) in content.
func firstHeading(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		if after, found := strings.CutPrefix(line, "# "); found {
			t := strings.TrimSpace(after)
			if t != "" {
				return t
			}
		}
	}
	return ""
}

// extractHeadingTags extracts unique tag suggestions from ## and ### headings.
// Returns at most 5 tags, lowercased and trimmed.
func extractHeadingTags(content string) []string {
	seen := make(map[string]bool)
	var tags []string
	for line := range strings.SplitSeq(content, "\n") {
		var heading string
		if after, ok := strings.CutPrefix(line, "### "); ok {
			heading = after
		} else if after, ok := strings.CutPrefix(line, "## "); ok {
			heading = after
		}
		if heading == "" {
			continue
		}
		tag := strings.ToLower(strings.TrimSpace(heading))
		// Skip very short or markdown-artifact headings.
		if len(tag) < 3 || strings.HasPrefix(tag, "#") {
			continue
		}
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
			if len(tags) >= 5 {
				break
			}
		}
	}
	return tags
}

// firstParagraph returns the first non-empty, non-heading paragraph of content,
// capped at 500 characters.
func firstParagraph(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	for p := range strings.SplitSeq(normalized, "\n\n") {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		runes := []rune(trimmed)
		if len(runes) > 500 {
			return string(runes[:500])
		}
		return trimmed
	}
	return ""
}

// unicodePunctuation contains Unicode punctuation characters to strip from words
// during keyword extraction (em dash, en dash, ellipsis, curly quotes, middle dot, bullet).
const unicodePunctuation = "\u2014\u2013\u2026\u201C\u201D\u2018\u2019\u00B7\u2022"

// stopWords is the set of common English words excluded from keyword extraction.
var stopWords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {},
	"in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "of": {},
	"with": {}, "by": {}, "from": {}, "is": {}, "it": {}, "that": {},
	"this": {}, "was": {}, "are": {}, "be": {}, "has": {}, "have": {},
	"had": {}, "not": {}, "no": {}, "do": {}, "does": {}, "did": {},
	"will": {}, "would": {}, "could": {}, "should": {}, "may": {},
	"might": {}, "can": {}, "shall": {}, "as": {}, "if": {}, "then": {},
	"than": {}, "so": {}, "up": {}, "out": {}, "about": {}, "into": {},
	"over": {}, "after": {}, "before": {}, "between": {}, "under": {},
	"again": {}, "there": {}, "here": {}, "when": {}, "where": {},
	"why": {}, "how": {}, "all": {}, "each": {}, "every": {}, "both": {},
	"few": {}, "more": {}, "most": {}, "other": {}, "some": {}, "such": {},
	"only": {}, "own": {}, "same": {}, "also": {}, "just": {}, "because": {},
	"its": {}, "i": {}, "me": {}, "my": {}, "we": {}, "our": {}, "you": {},
	"your": {}, "he": {}, "him": {}, "his": {}, "she": {}, "her": {},
	"they": {}, "them": {}, "their": {}, "what": {}, "which": {}, "who": {},
	"whom": {}, "been": {}, "being": {}, "were": {},
}

// extractKeywords returns the top 5 most frequent non-stop words from content.
func extractKeywords(content string) []string {
	freq := make(map[string]int)
	for word := range strings.FieldsSeq(content) {
		w := strings.ToLower(strings.Trim(word, ".,;:!?\"'()][}{"+unicodePunctuation))
		if len(w) < 3 {
			continue
		}
		if _, stop := stopWords[w]; stop {
			continue
		}
		freq[w]++
	}

	type wordCount struct {
		word  string
		count int
	}
	ranked := make([]wordCount, 0, len(freq))
	for w, c := range freq {
		ranked = append(ranked, wordCount{word: w, count: c})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].word < ranked[j].word
	})

	limit := min(5, len(ranked))
	keywords := make([]string, limit)
	for i := range limit {
		keywords[i] = ranked[i].word
	}
	return keywords
}

// metadataSubjects extracts subject tags from extractor metadata. Returns nil
// if no subjects are present. Caps at 5 entries, lowercased for consistency.
func metadataSubjects(metadata map[string]any) []string {
	v, ok := metadata["subjects"]
	if !ok {
		return nil
	}
	subjects, ok := v.([]string)
	if !ok || len(subjects) == 0 {
		return nil
	}

	limit := min(5, len(subjects))
	tags := make([]string, limit)
	for i := range limit {
		tags[i] = strings.ToLower(strings.TrimSpace(subjects[i]))
	}
	return tags
}

// TODO: implement language detection (currently hardcoded to "en").
func detectLanguage(_ string) string {
	return "en"
}
