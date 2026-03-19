// Package xlsx extracts text content from XLSX spreadsheet files.
package xlsx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"git.999.haus/chris/DocuMCP-go/internal/extractor"
)

const mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// Compile-time check that XLSXExtractor implements extractor.Extractor.
var _ extractor.Extractor = (*XLSXExtractor)(nil)

// XLSXExtractor extracts text content from XLSX spreadsheets.
//
//nolint:revive // exported stutter is intentional; renaming would be a breaking change
type XLSXExtractor struct{}

// New creates a new XLSXExtractor.
func New() *XLSXExtractor {
	return &XLSXExtractor{}
}

// Supports reports whether this extractor can handle the given MIME type.
func (e *XLSXExtractor) Supports(mime string) bool {
	return mime == mimeType
}

// Extract reads the XLSX file at filePath and returns its text content
// formatted as markdown tables.
func (e *XLSXExtractor) Extract(ctx context.Context, filePath string) (result *extractor.ExtractedContent, retErr error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening xlsx file: %w", err)
	}
	defer func() {
		if cErr := f.Close(); cErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("closing xlsx file: %w", cErr))
		}
	}()

	sheets := f.GetSheetList()

	var buf strings.Builder
	sheetNames := make([]string, 0, len(sheets))

	for i, sheet := range sheets {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("extracting xlsx: %w", ctx.Err())
		default:
		}

		sheetNames = append(sheetNames, sheet)

		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("reading sheet %q: %w", sheet, err)
		}

		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("## Sheet: ")
		buf.WriteString(sheet)
		buf.WriteString("\n\n")

		if len(rows) == 0 {
			continue
		}

		// Determine the maximum column count across all rows so every row
		// has a consistent number of columns in the markdown table.
		maxCols := 0
		for _, row := range rows {
			if len(row) > maxCols {
				maxCols = len(row)
			}
		}

		// Header row.
		writeRow(&buf, rows[0], maxCols)

		// Separator row.
		buf.WriteString("|")
		for range maxCols {
			buf.WriteString(" --- |")
		}
		buf.WriteString("\n")

		// Data rows.
		for _, row := range rows[1:] {
			writeRow(&buf, row, maxCols)
		}
	}

	text := buf.String()

	return &extractor.ExtractedContent{
		Content:   text,
		WordCount: len(strings.Fields(text)),
		Metadata: map[string]any{
			"sheet_count": len(sheets),
			"sheet_names": sheetNames,
		},
	}, nil
}

// writeRow writes a single pipe-separated markdown table row padded to numCols.
func writeRow(buf *strings.Builder, cells []string, numCols int) {
	buf.WriteString("|")
	for i := range numCols {
		buf.WriteString(" ")
		if i < len(cells) {
			buf.WriteString(cells[i])
		}
		buf.WriteString(" |")
	}
	buf.WriteString("\n")
}
