package search

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// mockIndexManager is a minimal mock for meilisearch.IndexManager.
// Only methods used by the search package are given real function fields;
// every other method panics so tests fail loudly if unexpected calls occur.
type mockIndexManager struct {
	addDocumentsWithContextFn   func(ctx context.Context, docs interface{}, opts *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)
	deleteDocumentWithContextFn func(ctx context.Context, id string, opts *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)
	searchWithContextFn         func(ctx context.Context, query string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error)
	getDocumentsWithContextFn   func(ctx context.Context, param *meilisearch.DocumentsQuery, resp *meilisearch.DocumentsResult) error
	updateSettingsWithContextFn func(ctx context.Context, req *meilisearch.Settings) (*meilisearch.TaskInfo, error)
}

func (m *mockIndexManager) AddDocumentsWithContext(ctx context.Context, docs interface{}, opts *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
	if m.addDocumentsWithContextFn != nil {
		return m.addDocumentsWithContextFn(ctx, docs, opts)
	}
	return &meilisearch.TaskInfo{TaskUID: 1}, nil
}
func (m *mockIndexManager) DeleteDocumentWithContext(ctx context.Context, id string, opts *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
	if m.deleteDocumentWithContextFn != nil {
		return m.deleteDocumentWithContextFn(ctx, id, opts)
	}
	return &meilisearch.TaskInfo{TaskUID: 1}, nil
}
func (m *mockIndexManager) SearchWithContext(ctx context.Context, query string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
	if m.searchWithContextFn != nil {
		return m.searchWithContextFn(ctx, query, req)
	}
	return &meilisearch.SearchResponse{}, nil
}
func (m *mockIndexManager) GetDocumentsWithContext(ctx context.Context, param *meilisearch.DocumentsQuery, resp *meilisearch.DocumentsResult) error {
	if m.getDocumentsWithContextFn != nil {
		return m.getDocumentsWithContextFn(ctx, param, resp)
	}
	return nil
}
func (m *mockIndexManager) UpdateSettingsWithContext(ctx context.Context, req *meilisearch.Settings) (*meilisearch.TaskInfo, error) {
	if m.updateSettingsWithContextFn != nil {
		return m.updateSettingsWithContextFn(ctx, req)
	}
	return &meilisearch.TaskInfo{TaskUID: 1}, nil
}

// --- Stub methods for IndexManager (panic on unexpected calls) ---

func (m *mockIndexManager) GetIndexReader() meilisearch.IndexReader        { panic("stub") }
func (m *mockIndexManager) GetTaskReader() meilisearch.TaskReader          { panic("stub") }
func (m *mockIndexManager) GetDocumentManager() meilisearch.DocumentManager { panic("stub") }
func (m *mockIndexManager) GetDocumentReader() meilisearch.DocumentReader  { panic("stub") }
func (m *mockIndexManager) GetSettingsManager() meilisearch.SettingsManager { panic("stub") }
func (m *mockIndexManager) GetSettingsReader() meilisearch.SettingsReader  { panic("stub") }
func (m *mockIndexManager) GetSearch() meilisearch.SearchReader            { panic("stub") }

func (m *mockIndexManager) FetchInfo() (*meilisearch.IndexResult, error)                             { panic("stub") }
func (m *mockIndexManager) FetchInfoWithContext(context.Context) (*meilisearch.IndexResult, error)   { panic("stub") }
func (m *mockIndexManager) FetchPrimaryKey() (*string, error)                                         { panic("stub") }
func (m *mockIndexManager) FetchPrimaryKeyWithContext(context.Context) (*string, error)               { panic("stub") }
func (m *mockIndexManager) GetStats() (*meilisearch.StatsIndex, error)                                { panic("stub") }
func (m *mockIndexManager) GetStatsWithContext(context.Context) (*meilisearch.StatsIndex, error)      { panic("stub") }
func (m *mockIndexManager) UpdateIndex(*meilisearch.UpdateIndexRequestParams) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) UpdateIndexWithContext(context.Context, *meilisearch.UpdateIndexRequestParams) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) Delete(string) (bool, error)                                               { panic("stub") }
func (m *mockIndexManager) DeleteWithContext(context.Context, string) (bool, error)                   { panic("stub") }
func (m *mockIndexManager) Compact() (*meilisearch.TaskInfo, error)                                   { panic("stub") }
func (m *mockIndexManager) CompactWithContext(context.Context) (*meilisearch.TaskInfo, error)          { panic("stub") }

func (m *mockIndexManager) GetTask(int64) (*meilisearch.Task, error)                                  { panic("stub") }
func (m *mockIndexManager) GetTaskWithContext(context.Context, int64) (*meilisearch.Task, error)       { panic("stub") }
func (m *mockIndexManager) GetTasks(*meilisearch.TasksQuery) (*meilisearch.TaskResult, error)          { panic("stub") }
func (m *mockIndexManager) GetTasksWithContext(context.Context, *meilisearch.TasksQuery) (*meilisearch.TaskResult, error) { panic("stub") }
func (m *mockIndexManager) WaitForTask(int64, time.Duration) (*meilisearch.Task, error)                { panic("stub") }
func (m *mockIndexManager) WaitForTaskWithContext(context.Context, int64, time.Duration) (*meilisearch.Task, error) { panic("stub") }

func (m *mockIndexManager) GetDocument(string, *meilisearch.DocumentQuery, interface{}) error          { panic("stub") }
func (m *mockIndexManager) GetDocumentWithContext(context.Context, string, *meilisearch.DocumentQuery, interface{}) error { panic("stub") }
func (m *mockIndexManager) GetDocuments(*meilisearch.DocumentsQuery, *meilisearch.DocumentsResult) error { panic("stub") }

func (m *mockIndexManager) AddDocuments(interface{}, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)                                                       { panic("stub") }
func (m *mockIndexManager) AddDocumentsInBatches(interface{}, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error)                                        { panic("stub") }
func (m *mockIndexManager) AddDocumentsInBatchesWithContext(context.Context, interface{}, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error)             { panic("stub") }
func (m *mockIndexManager) AddDocumentsCsv([]byte, *meilisearch.CsvDocumentsQuery) (*meilisearch.TaskInfo, error)                                                      { panic("stub") }
func (m *mockIndexManager) AddDocumentsCsvWithContext(context.Context, []byte, *meilisearch.CsvDocumentsQuery) (*meilisearch.TaskInfo, error)                           { panic("stub") }
func (m *mockIndexManager) AddDocumentsCsvInBatches([]byte, int, *meilisearch.CsvDocumentsQuery) ([]meilisearch.TaskInfo, error)                                       { panic("stub") }
func (m *mockIndexManager) AddDocumentsCsvInBatchesWithContext(context.Context, []byte, int, *meilisearch.CsvDocumentsQuery) ([]meilisearch.TaskInfo, error)            { panic("stub") }
func (m *mockIndexManager) AddDocumentsCsvFromReaderInBatches(io.Reader, int, *meilisearch.CsvDocumentsQuery) ([]meilisearch.TaskInfo, error)                           { panic("stub") }
func (m *mockIndexManager) AddDocumentsCsvFromReaderInBatchesWithContext(context.Context, io.Reader, int, *meilisearch.CsvDocumentsQuery) ([]meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) AddDocumentsCsvFromReader(io.Reader, *meilisearch.CsvDocumentsQuery) (*meilisearch.TaskInfo, error)                                         { panic("stub") }
func (m *mockIndexManager) AddDocumentsCsvFromReaderWithContext(context.Context, io.Reader, *meilisearch.CsvDocumentsQuery) (*meilisearch.TaskInfo, error)              { panic("stub") }
func (m *mockIndexManager) AddDocumentsNdjson([]byte, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)                                                     { panic("stub") }
func (m *mockIndexManager) AddDocumentsNdjsonWithContext(context.Context, []byte, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)                          { panic("stub") }
func (m *mockIndexManager) AddDocumentsNdjsonInBatches([]byte, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error)                                      { panic("stub") }
func (m *mockIndexManager) AddDocumentsNdjsonInBatchesWithContext(context.Context, []byte, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error)           { panic("stub") }
func (m *mockIndexManager) AddDocumentsNdjsonFromReader(io.Reader, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)                                        { panic("stub") }
func (m *mockIndexManager) AddDocumentsNdjsonFromReaderWithContext(context.Context, io.Reader, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)             { panic("stub") }
func (m *mockIndexManager) AddDocumentsNdjsonFromReaderInBatches(io.Reader, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error)                          { panic("stub") }
func (m *mockIndexManager) AddDocumentsNdjsonFromReaderInBatchesWithContext(context.Context, io.Reader, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error) { panic("stub") }

func (m *mockIndexManager) UpdateDocuments(interface{}, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)                            { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsWithContext(context.Context, interface{}, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsInBatches(interface{}, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error)              { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsInBatchesWithContext(context.Context, interface{}, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsCsv([]byte, *meilisearch.CsvDocumentsQuery) (*meilisearch.TaskInfo, error)                            { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsCsvWithContext(context.Context, []byte, *meilisearch.CsvDocumentsQuery) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsCsvInBatches([]byte, int, *meilisearch.CsvDocumentsQuery) ([]meilisearch.TaskInfo, error)              { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsCsvInBatchesWithContext(context.Context, []byte, int, *meilisearch.CsvDocumentsQuery) ([]meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsNdjson([]byte, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)                            { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsNdjsonWithContext(context.Context, []byte, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsNdjsonInBatches([]byte, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error)              { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsNdjsonInBatchesWithContext(context.Context, []byte, int, *meilisearch.DocumentOptions) ([]meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsByFunction(*meilisearch.UpdateDocumentByFunctionRequest) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) UpdateDocumentsByFunctionWithContext(context.Context, *meilisearch.UpdateDocumentByFunctionRequest) (*meilisearch.TaskInfo, error) { panic("stub") }

func (m *mockIndexManager) DeleteDocument(string, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)           { panic("stub") }
func (m *mockIndexManager) DeleteDocuments([]string, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error)         { panic("stub") }
func (m *mockIndexManager) DeleteDocumentsWithContext(context.Context, []string, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) DeleteDocumentsByFilter(interface{}, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) DeleteDocumentsByFilterWithContext(context.Context, interface{}, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) DeleteAllDocuments(*meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) DeleteAllDocumentsWithContext(context.Context, *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) { panic("stub") }

// Settings stubs
func (m *mockIndexManager) GetSettings() (*meilisearch.Settings, error)                                                   { panic("stub") }
func (m *mockIndexManager) GetSettingsWithContext(context.Context) (*meilisearch.Settings, error)                          { panic("stub") }
func (m *mockIndexManager) UpdateSettings(*meilisearch.Settings) (*meilisearch.TaskInfo, error)                            { panic("stub") }
func (m *mockIndexManager) ResetSettings() (*meilisearch.TaskInfo, error)                                                  { panic("stub") }
func (m *mockIndexManager) ResetSettingsWithContext(context.Context) (*meilisearch.TaskInfo, error)                        { panic("stub") }
func (m *mockIndexManager) UpdateRankingRules(*[]string) (*meilisearch.TaskInfo, error)                                    { panic("stub") }
func (m *mockIndexManager) UpdateRankingRulesWithContext(context.Context, *[]string) (*meilisearch.TaskInfo, error)        { panic("stub") }
func (m *mockIndexManager) ResetRankingRules() (*meilisearch.TaskInfo, error)                                              { panic("stub") }
func (m *mockIndexManager) ResetRankingRulesWithContext(context.Context) (*meilisearch.TaskInfo, error)                    { panic("stub") }
func (m *mockIndexManager) GetRankingRules() (*[]string, error)                                                            { panic("stub") }
func (m *mockIndexManager) GetRankingRulesWithContext(context.Context) (*[]string, error)                                  { panic("stub") }
func (m *mockIndexManager) GetDistinctAttribute() (*string, error)                                                         { panic("stub") }
func (m *mockIndexManager) GetDistinctAttributeWithContext(context.Context) (*string, error)                               { panic("stub") }
func (m *mockIndexManager) UpdateDistinctAttribute(string) (*meilisearch.TaskInfo, error)                                  { panic("stub") }
func (m *mockIndexManager) UpdateDistinctAttributeWithContext(context.Context, string) (*meilisearch.TaskInfo, error)      { panic("stub") }
func (m *mockIndexManager) ResetDistinctAttribute() (*meilisearch.TaskInfo, error)                                         { panic("stub") }
func (m *mockIndexManager) ResetDistinctAttributeWithContext(context.Context) (*meilisearch.TaskInfo, error)               { panic("stub") }
func (m *mockIndexManager) GetSearchableAttributes() (*[]string, error)                                                     { panic("stub") }
func (m *mockIndexManager) GetSearchableAttributesWithContext(context.Context) (*[]string, error)                           { panic("stub") }
func (m *mockIndexManager) UpdateSearchableAttributes(*[]string) (*meilisearch.TaskInfo, error)                            { panic("stub") }
func (m *mockIndexManager) UpdateSearchableAttributesWithContext(context.Context, *[]string) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetSearchableAttributes() (*meilisearch.TaskInfo, error)                                      { panic("stub") }
func (m *mockIndexManager) ResetSearchableAttributesWithContext(context.Context) (*meilisearch.TaskInfo, error)            { panic("stub") }
func (m *mockIndexManager) GetDisplayedAttributes() (*[]string, error)                                                     { panic("stub") }
func (m *mockIndexManager) GetDisplayedAttributesWithContext(context.Context) (*[]string, error)                           { panic("stub") }
func (m *mockIndexManager) UpdateDisplayedAttributes(*[]string) (*meilisearch.TaskInfo, error)                             { panic("stub") }
func (m *mockIndexManager) UpdateDisplayedAttributesWithContext(context.Context, *[]string) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetDisplayedAttributes() (*meilisearch.TaskInfo, error)                                       { panic("stub") }
func (m *mockIndexManager) ResetDisplayedAttributesWithContext(context.Context) (*meilisearch.TaskInfo, error)             { panic("stub") }
func (m *mockIndexManager) GetStopWords() (*[]string, error)                                                               { panic("stub") }
func (m *mockIndexManager) GetStopWordsWithContext(context.Context) (*[]string, error)                                     { panic("stub") }
func (m *mockIndexManager) UpdateStopWords(*[]string) (*meilisearch.TaskInfo, error)                                       { panic("stub") }
func (m *mockIndexManager) UpdateStopWordsWithContext(context.Context, *[]string) (*meilisearch.TaskInfo, error)           { panic("stub") }
func (m *mockIndexManager) ResetStopWords() (*meilisearch.TaskInfo, error)                                                 { panic("stub") }
func (m *mockIndexManager) ResetStopWordsWithContext(context.Context) (*meilisearch.TaskInfo, error)                       { panic("stub") }
func (m *mockIndexManager) GetSynonyms() (*map[string][]string, error)                                                     { panic("stub") }
func (m *mockIndexManager) GetSynonymsWithContext(context.Context) (*map[string][]string, error)                           { panic("stub") }
func (m *mockIndexManager) UpdateSynonyms(*map[string][]string) (*meilisearch.TaskInfo, error)                             { panic("stub") }
func (m *mockIndexManager) UpdateSynonymsWithContext(context.Context, *map[string][]string) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetSynonyms() (*meilisearch.TaskInfo, error)                                                  { panic("stub") }
func (m *mockIndexManager) ResetSynonymsWithContext(context.Context) (*meilisearch.TaskInfo, error)                        { panic("stub") }
func (m *mockIndexManager) GetFilterableAttributes() (*[]interface{}, error)                                                { panic("stub") }
func (m *mockIndexManager) GetFilterableAttributesWithContext(context.Context) (*[]interface{}, error)                      { panic("stub") }
func (m *mockIndexManager) UpdateFilterableAttributes(*[]interface{}) (*meilisearch.TaskInfo, error)                       { panic("stub") }
func (m *mockIndexManager) UpdateFilterableAttributesWithContext(context.Context, *[]interface{}) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetFilterableAttributes() (*meilisearch.TaskInfo, error)                                      { panic("stub") }
func (m *mockIndexManager) ResetFilterableAttributesWithContext(context.Context) (*meilisearch.TaskInfo, error)            { panic("stub") }
func (m *mockIndexManager) GetSortableAttributes() (*[]string, error)                                                      { panic("stub") }
func (m *mockIndexManager) GetSortableAttributesWithContext(context.Context) (*[]string, error)                            { panic("stub") }
func (m *mockIndexManager) UpdateSortableAttributes(*[]string) (*meilisearch.TaskInfo, error)                              { panic("stub") }
func (m *mockIndexManager) UpdateSortableAttributesWithContext(context.Context, *[]string) (*meilisearch.TaskInfo, error)  { panic("stub") }
func (m *mockIndexManager) ResetSortableAttributes() (*meilisearch.TaskInfo, error)                                        { panic("stub") }
func (m *mockIndexManager) ResetSortableAttributesWithContext(context.Context) (*meilisearch.TaskInfo, error)              { panic("stub") }
func (m *mockIndexManager) GetTypoTolerance() (*meilisearch.TypoTolerance, error)                                          { panic("stub") }
func (m *mockIndexManager) GetTypoToleranceWithContext(context.Context) (*meilisearch.TypoTolerance, error)                { panic("stub") }
func (m *mockIndexManager) UpdateTypoTolerance(*meilisearch.TypoTolerance) (*meilisearch.TaskInfo, error)                  { panic("stub") }
func (m *mockIndexManager) UpdateTypoToleranceWithContext(context.Context, *meilisearch.TypoTolerance) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetTypoTolerance() (*meilisearch.TaskInfo, error)                                             { panic("stub") }
func (m *mockIndexManager) ResetTypoToleranceWithContext(context.Context) (*meilisearch.TaskInfo, error)                   { panic("stub") }
func (m *mockIndexManager) GetPagination() (*meilisearch.Pagination, error)                                                { panic("stub") }
func (m *mockIndexManager) GetPaginationWithContext(context.Context) (*meilisearch.Pagination, error)                      { panic("stub") }
func (m *mockIndexManager) UpdatePagination(*meilisearch.Pagination) (*meilisearch.TaskInfo, error)                        { panic("stub") }
func (m *mockIndexManager) UpdatePaginationWithContext(context.Context, *meilisearch.Pagination) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetPagination() (*meilisearch.TaskInfo, error)                                                { panic("stub") }
func (m *mockIndexManager) ResetPaginationWithContext(context.Context) (*meilisearch.TaskInfo, error)                      { panic("stub") }
func (m *mockIndexManager) GetFaceting() (*meilisearch.Faceting, error)                                                    { panic("stub") }
func (m *mockIndexManager) GetFacetingWithContext(context.Context) (*meilisearch.Faceting, error)                          { panic("stub") }
func (m *mockIndexManager) UpdateFaceting(*meilisearch.Faceting) (*meilisearch.TaskInfo, error)                            { panic("stub") }
func (m *mockIndexManager) UpdateFacetingWithContext(context.Context, *meilisearch.Faceting) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetFaceting() (*meilisearch.TaskInfo, error)                                                  { panic("stub") }
func (m *mockIndexManager) ResetFacetingWithContext(context.Context) (*meilisearch.TaskInfo, error)                        { panic("stub") }
func (m *mockIndexManager) GetEmbedders() (map[string]meilisearch.Embedder, error)                                         { panic("stub") }
func (m *mockIndexManager) GetEmbeddersWithContext(context.Context) (map[string]meilisearch.Embedder, error)               { panic("stub") }
func (m *mockIndexManager) UpdateEmbedders(map[string]meilisearch.Embedder) (*meilisearch.TaskInfo, error)                 { panic("stub") }
func (m *mockIndexManager) UpdateEmbeddersWithContext(context.Context, map[string]meilisearch.Embedder) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetEmbedders() (*meilisearch.TaskInfo, error)                                                 { panic("stub") }
func (m *mockIndexManager) ResetEmbeddersWithContext(context.Context) (*meilisearch.TaskInfo, error)                       { panic("stub") }
func (m *mockIndexManager) GetSearchCutoffMs() (int64, error)                                                              { panic("stub") }
func (m *mockIndexManager) GetSearchCutoffMsWithContext(context.Context) (int64, error)                                    { panic("stub") }
func (m *mockIndexManager) UpdateSearchCutoffMs(int64) (*meilisearch.TaskInfo, error)                                      { panic("stub") }
func (m *mockIndexManager) UpdateSearchCutoffMsWithContext(context.Context, int64) (*meilisearch.TaskInfo, error)          { panic("stub") }
func (m *mockIndexManager) ResetSearchCutoffMs() (*meilisearch.TaskInfo, error)                                            { panic("stub") }
func (m *mockIndexManager) ResetSearchCutoffMsWithContext(context.Context) (*meilisearch.TaskInfo, error)                  { panic("stub") }
func (m *mockIndexManager) GetSeparatorTokens() ([]string, error)                                                          { panic("stub") }
func (m *mockIndexManager) GetSeparatorTokensWithContext(context.Context) ([]string, error)                                { panic("stub") }
func (m *mockIndexManager) UpdateSeparatorTokens([]string) (*meilisearch.TaskInfo, error)                                  { panic("stub") }
func (m *mockIndexManager) UpdateSeparatorTokensWithContext(context.Context, []string) (*meilisearch.TaskInfo, error)      { panic("stub") }
func (m *mockIndexManager) ResetSeparatorTokens() (*meilisearch.TaskInfo, error)                                           { panic("stub") }
func (m *mockIndexManager) ResetSeparatorTokensWithContext(context.Context) (*meilisearch.TaskInfo, error)                 { panic("stub") }
func (m *mockIndexManager) GetNonSeparatorTokens() ([]string, error)                                                       { panic("stub") }
func (m *mockIndexManager) GetNonSeparatorTokensWithContext(context.Context) ([]string, error)                             { panic("stub") }
func (m *mockIndexManager) UpdateNonSeparatorTokens([]string) (*meilisearch.TaskInfo, error)                               { panic("stub") }
func (m *mockIndexManager) UpdateNonSeparatorTokensWithContext(context.Context, []string) (*meilisearch.TaskInfo, error)   { panic("stub") }
func (m *mockIndexManager) ResetNonSeparatorTokens() (*meilisearch.TaskInfo, error)                                        { panic("stub") }
func (m *mockIndexManager) ResetNonSeparatorTokensWithContext(context.Context) (*meilisearch.TaskInfo, error)              { panic("stub") }
func (m *mockIndexManager) GetDictionary() ([]string, error)                                                               { panic("stub") }
func (m *mockIndexManager) GetDictionaryWithContext(context.Context) ([]string, error)                                     { panic("stub") }
func (m *mockIndexManager) UpdateDictionary([]string) (*meilisearch.TaskInfo, error)                                       { panic("stub") }
func (m *mockIndexManager) UpdateDictionaryWithContext(context.Context, []string) (*meilisearch.TaskInfo, error)           { panic("stub") }
func (m *mockIndexManager) ResetDictionary() (*meilisearch.TaskInfo, error)                                                { panic("stub") }
func (m *mockIndexManager) ResetDictionaryWithContext(context.Context) (*meilisearch.TaskInfo, error)                      { panic("stub") }
func (m *mockIndexManager) GetProximityPrecision() (meilisearch.ProximityPrecisionType, error)                             { panic("stub") }
func (m *mockIndexManager) GetProximityPrecisionWithContext(context.Context) (meilisearch.ProximityPrecisionType, error)   { panic("stub") }
func (m *mockIndexManager) UpdateProximityPrecision(meilisearch.ProximityPrecisionType) (*meilisearch.TaskInfo, error)     { panic("stub") }
func (m *mockIndexManager) UpdateProximityPrecisionWithContext(context.Context, meilisearch.ProximityPrecisionType) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetProximityPrecision() (*meilisearch.TaskInfo, error)                                        { panic("stub") }
func (m *mockIndexManager) ResetProximityPrecisionWithContext(context.Context) (*meilisearch.TaskInfo, error)              { panic("stub") }
func (m *mockIndexManager) GetLocalizedAttributes() ([]*meilisearch.LocalizedAttributes, error)                            { panic("stub") }
func (m *mockIndexManager) GetLocalizedAttributesWithContext(context.Context) ([]*meilisearch.LocalizedAttributes, error)  { panic("stub") }
func (m *mockIndexManager) UpdateLocalizedAttributes([]*meilisearch.LocalizedAttributes) (*meilisearch.TaskInfo, error)    { panic("stub") }
func (m *mockIndexManager) UpdateLocalizedAttributesWithContext(context.Context, []*meilisearch.LocalizedAttributes) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockIndexManager) ResetLocalizedAttributes() (*meilisearch.TaskInfo, error)                                       { panic("stub") }
func (m *mockIndexManager) ResetLocalizedAttributesWithContext(context.Context) (*meilisearch.TaskInfo, error)             { panic("stub") }
func (m *mockIndexManager) GetPrefixSearch() (*string, error)                                                              { panic("stub") }
func (m *mockIndexManager) GetPrefixSearchWithContext(context.Context) (*string, error)                                    { panic("stub") }
func (m *mockIndexManager) UpdatePrefixSearch(string) (*meilisearch.TaskInfo, error)                                       { panic("stub") }
func (m *mockIndexManager) UpdatePrefixSearchWithContext(context.Context, string) (*meilisearch.TaskInfo, error)           { panic("stub") }
func (m *mockIndexManager) ResetPrefixSearch() (*meilisearch.TaskInfo, error)                                              { panic("stub") }
func (m *mockIndexManager) ResetPrefixSearchWithContext(context.Context) (*meilisearch.TaskInfo, error)                    { panic("stub") }
func (m *mockIndexManager) GetFacetSearch() (bool, error)                                                                  { panic("stub") }
func (m *mockIndexManager) GetFacetSearchWithContext(context.Context) (bool, error)                                        { panic("stub") }
func (m *mockIndexManager) UpdateFacetSearch(bool) (*meilisearch.TaskInfo, error)                                          { panic("stub") }
func (m *mockIndexManager) UpdateFacetSearchWithContext(context.Context, bool) (*meilisearch.TaskInfo, error)              { panic("stub") }
func (m *mockIndexManager) ResetFacetSearch() (*meilisearch.TaskInfo, error)                                               { panic("stub") }
func (m *mockIndexManager) ResetFacetSearchWithContext(context.Context) (*meilisearch.TaskInfo, error)                     { panic("stub") }

// Search stubs
func (m *mockIndexManager) Search(string, *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) { panic("stub") }
func (m *mockIndexManager) SearchRaw(string, *meilisearch.SearchRequest) (*json.RawMessage, error)         { panic("stub") }
func (m *mockIndexManager) SearchRawWithContext(context.Context, string, *meilisearch.SearchRequest) (*json.RawMessage, error) { panic("stub") }
func (m *mockIndexManager) FacetSearch(*meilisearch.FacetSearchRequest) (*json.RawMessage, error)          { panic("stub") }
func (m *mockIndexManager) FacetSearchWithContext(context.Context, *meilisearch.FacetSearchRequest) (*json.RawMessage, error) { panic("stub") }
func (m *mockIndexManager) SearchSimilarDocuments(*meilisearch.SimilarDocumentQuery, *meilisearch.SimilarDocumentResult) error { panic("stub") }
func (m *mockIndexManager) SearchSimilarDocumentsWithContext(context.Context, *meilisearch.SimilarDocumentQuery, *meilisearch.SimilarDocumentResult) error { panic("stub") }

// ---------------------------------------------------------------------------
// mockServiceManager
// ---------------------------------------------------------------------------

type mockServiceManager struct {
	indexFn                  func(uid string) meilisearch.IndexManager
	createIndexWithContextFn func(ctx context.Context, config *meilisearch.IndexConfig) (*meilisearch.TaskInfo, error)
	isHealthyFn              func() bool
	waitForTaskWithContextFn func(ctx context.Context, taskUID int64, interval time.Duration) (*meilisearch.Task, error)
	multiSearchWithContextFn func(ctx context.Context, queries *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error)
}

func (m *mockServiceManager) Index(uid string) meilisearch.IndexManager {
	if m.indexFn != nil {
		return m.indexFn(uid)
	}
	return &mockIndexManager{}
}
func (m *mockServiceManager) CreateIndexWithContext(ctx context.Context, config *meilisearch.IndexConfig) (*meilisearch.TaskInfo, error) {
	if m.createIndexWithContextFn != nil {
		return m.createIndexWithContextFn(ctx, config)
	}
	return &meilisearch.TaskInfo{TaskUID: 1}, nil
}
func (m *mockServiceManager) IsHealthy() bool {
	if m.isHealthyFn != nil {
		return m.isHealthyFn()
	}
	return true
}
func (m *mockServiceManager) WaitForTaskWithContext(ctx context.Context, taskUID int64, interval time.Duration) (*meilisearch.Task, error) {
	if m.waitForTaskWithContextFn != nil {
		return m.waitForTaskWithContextFn(ctx, taskUID, interval)
	}
	return &meilisearch.Task{}, nil
}
func (m *mockServiceManager) MultiSearchWithContext(ctx context.Context, queries *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) {
	if m.multiSearchWithContextFn != nil {
		return m.multiSearchWithContextFn(ctx, queries)
	}
	return &meilisearch.MultiSearchResponse{}, nil
}

// --- ServiceManager stubs ---
func (m *mockServiceManager) ServiceReader() meilisearch.ServiceReader   { panic("stub") }
func (m *mockServiceManager) TaskManager() meilisearch.TaskManager       { panic("stub") }
func (m *mockServiceManager) TaskReader() meilisearch.TaskReader         { panic("stub") }
func (m *mockServiceManager) KeyManager() meilisearch.KeyManager         { panic("stub") }
func (m *mockServiceManager) KeyReader() meilisearch.KeyReader           { panic("stub") }
func (m *mockServiceManager) ChatManager() meilisearch.ChatManager       { panic("stub") }
func (m *mockServiceManager) ChatReader() meilisearch.ChatReader         { panic("stub") }
func (m *mockServiceManager) WebhookManager() meilisearch.WebhookManager { panic("stub") }
func (m *mockServiceManager) WebhookReader() meilisearch.WebhookReader   { panic("stub") }

func (m *mockServiceManager) GetIndex(string) (*meilisearch.IndexResult, error)                               { panic("stub") }
func (m *mockServiceManager) GetIndexWithContext(context.Context, string) (*meilisearch.IndexResult, error)   { panic("stub") }
func (m *mockServiceManager) GetRawIndex(string) (map[string]interface{}, error)                              { panic("stub") }
func (m *mockServiceManager) GetRawIndexWithContext(context.Context, string) (map[string]interface{}, error)  { panic("stub") }
func (m *mockServiceManager) ListIndexes(*meilisearch.IndexesQuery) (*meilisearch.IndexesResults, error)      { panic("stub") }
func (m *mockServiceManager) ListIndexesWithContext(context.Context, *meilisearch.IndexesQuery) (*meilisearch.IndexesResults, error) { panic("stub") }
func (m *mockServiceManager) GetRawIndexes(*meilisearch.IndexesQuery) (map[string]interface{}, error)          { panic("stub") }
func (m *mockServiceManager) GetRawIndexesWithContext(context.Context, *meilisearch.IndexesQuery) (map[string]interface{}, error) { panic("stub") }
func (m *mockServiceManager) GetBatches(*meilisearch.BatchesQuery) (*meilisearch.BatchesResults, error)        { panic("stub") }
func (m *mockServiceManager) GetBatchesWithContext(context.Context, *meilisearch.BatchesQuery) (*meilisearch.BatchesResults, error) { panic("stub") }
func (m *mockServiceManager) GetBatch(int) (*meilisearch.Batch, error)                                         { panic("stub") }
func (m *mockServiceManager) GetBatchWithContext(context.Context, int) (*meilisearch.Batch, error)              { panic("stub") }
func (m *mockServiceManager) GetNetwork() (*meilisearch.Network, error)                                        { panic("stub") }
func (m *mockServiceManager) GetNetworkWithContext(context.Context) (*meilisearch.Network, error)               { panic("stub") }
func (m *mockServiceManager) MultiSearch(*meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) { panic("stub") }
func (m *mockServiceManager) GetStats() (*meilisearch.Stats, error)                                           { panic("stub") }
func (m *mockServiceManager) GetStatsWithContext(context.Context) (*meilisearch.Stats, error)                 { panic("stub") }
func (m *mockServiceManager) Version() (*meilisearch.Version, error)                                          { panic("stub") }
func (m *mockServiceManager) VersionWithContext(context.Context) (*meilisearch.Version, error)                { panic("stub") }
func (m *mockServiceManager) Health() (*meilisearch.Health, error)                                             { panic("stub") }
func (m *mockServiceManager) HealthWithContext(context.Context) (*meilisearch.Health, error)                   { panic("stub") }
func (m *mockServiceManager) CreateIndex(*meilisearch.IndexConfig) (*meilisearch.TaskInfo, error)             { panic("stub") }
func (m *mockServiceManager) DeleteIndex(string) (*meilisearch.TaskInfo, error)                                { panic("stub") }
func (m *mockServiceManager) DeleteIndexWithContext(context.Context, string) (*meilisearch.TaskInfo, error)    { panic("stub") }
func (m *mockServiceManager) SwapIndexes([]*meilisearch.SwapIndexesParams) (*meilisearch.TaskInfo, error)      { panic("stub") }
func (m *mockServiceManager) SwapIndexesWithContext(context.Context, []*meilisearch.SwapIndexesParams) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockServiceManager) GenerateTenantToken(string, map[string]interface{}, *meilisearch.TenantTokenOptions) (string, error) { panic("stub") }
func (m *mockServiceManager) CreateDump() (*meilisearch.TaskInfo, error)                                       { panic("stub") }
func (m *mockServiceManager) CreateDumpWithContext(context.Context) (*meilisearch.TaskInfo, error)             { panic("stub") }
func (m *mockServiceManager) CreateSnapshot() (*meilisearch.TaskInfo, error)                                   { panic("stub") }
func (m *mockServiceManager) CreateSnapshotWithContext(context.Context) (*meilisearch.TaskInfo, error)         { panic("stub") }
func (m *mockServiceManager) ExperimentalFeatures() *meilisearch.ExperimentalFeatures                         { panic("stub") }
func (m *mockServiceManager) Export(*meilisearch.ExportParams) (*meilisearch.TaskInfo, error)                  { panic("stub") }
func (m *mockServiceManager) ExportWithContext(context.Context, *meilisearch.ExportParams) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockServiceManager) UpdateNetwork(*meilisearch.UpdateNetworkRequest) (any, error)                     { panic("stub") }
func (m *mockServiceManager) UpdateNetworkWithContext(context.Context, *meilisearch.UpdateNetworkRequest) (any, error) { panic("stub") }
func (m *mockServiceManager) Close()                                                                           {}
func (m *mockServiceManager) GetTask(int64) (*meilisearch.Task, error)                                        { panic("stub") }
func (m *mockServiceManager) GetTaskWithContext(context.Context, int64) (*meilisearch.Task, error)             { panic("stub") }
func (m *mockServiceManager) GetTasks(*meilisearch.TasksQuery) (*meilisearch.TaskResult, error)                { panic("stub") }
func (m *mockServiceManager) GetTasksWithContext(context.Context, *meilisearch.TasksQuery) (*meilisearch.TaskResult, error) { panic("stub") }
func (m *mockServiceManager) WaitForTask(int64, time.Duration) (*meilisearch.Task, error)                      { panic("stub") }
func (m *mockServiceManager) CancelTasks(*meilisearch.CancelTasksQuery) (*meilisearch.TaskInfo, error)        { panic("stub") }
func (m *mockServiceManager) CancelTasksWithContext(context.Context, *meilisearch.CancelTasksQuery) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockServiceManager) DeleteTasks(*meilisearch.DeleteTasksQuery) (*meilisearch.TaskInfo, error)         { panic("stub") }
func (m *mockServiceManager) DeleteTasksWithContext(context.Context, *meilisearch.DeleteTasksQuery) (*meilisearch.TaskInfo, error) { panic("stub") }
func (m *mockServiceManager) GetKey(string) (*meilisearch.Key, error)                                          { panic("stub") }
func (m *mockServiceManager) GetKeyWithContext(context.Context, string) (*meilisearch.Key, error)              { panic("stub") }
func (m *mockServiceManager) GetKeys(*meilisearch.KeysQuery) (*meilisearch.KeysResults, error)                 { panic("stub") }
func (m *mockServiceManager) GetKeysWithContext(context.Context, *meilisearch.KeysQuery) (*meilisearch.KeysResults, error) { panic("stub") }
func (m *mockServiceManager) CreateKey(*meilisearch.Key) (*meilisearch.Key, error)                             { panic("stub") }
func (m *mockServiceManager) CreateKeyWithContext(context.Context, *meilisearch.Key) (*meilisearch.Key, error) { panic("stub") }
func (m *mockServiceManager) UpdateKey(string, *meilisearch.Key) (*meilisearch.Key, error)                     { panic("stub") }
func (m *mockServiceManager) UpdateKeyWithContext(context.Context, string, *meilisearch.Key) (*meilisearch.Key, error) { panic("stub") }
func (m *mockServiceManager) DeleteKey(string) (bool, error)                                                    { panic("stub") }
func (m *mockServiceManager) DeleteKeyWithContext(context.Context, string) (bool, error)                       { panic("stub") }
func (m *mockServiceManager) ChatCompletionStream(string, *meilisearch.ChatCompletionQuery) (*meilisearch.Stream[*meilisearch.ChatCompletionStreamChunk], error) { panic("stub") }
func (m *mockServiceManager) ChatCompletionStreamWithContext(context.Context, string, *meilisearch.ChatCompletionQuery) (*meilisearch.Stream[*meilisearch.ChatCompletionStreamChunk], error) { panic("stub") }
func (m *mockServiceManager) ListChatWorkspaces(*meilisearch.ListChatWorkSpaceQuery) (*meilisearch.ListChatWorkspace, error) { panic("stub") }
func (m *mockServiceManager) ListChatWorkspacesWithContext(context.Context, *meilisearch.ListChatWorkSpaceQuery) (*meilisearch.ListChatWorkspace, error) { panic("stub") }
func (m *mockServiceManager) GetChatWorkspace(string) (*meilisearch.ChatWorkspace, error)                      { panic("stub") }
func (m *mockServiceManager) GetChatWorkspaceWithContext(context.Context, string) (*meilisearch.ChatWorkspace, error) { panic("stub") }
func (m *mockServiceManager) GetChatWorkspaceSettings(string) (*meilisearch.ChatWorkspaceSettings, error)      { panic("stub") }
func (m *mockServiceManager) GetChatWorkspaceSettingsWithContext(context.Context, string) (*meilisearch.ChatWorkspaceSettings, error) { panic("stub") }
func (m *mockServiceManager) UpdateChatWorkspace(string, *meilisearch.ChatWorkspaceSettings) (*meilisearch.ChatWorkspaceSettings, error) { panic("stub") }
func (m *mockServiceManager) UpdateChatWorkspaceWithContext(context.Context, string, *meilisearch.ChatWorkspaceSettings) (*meilisearch.ChatWorkspaceSettings, error) { panic("stub") }
func (m *mockServiceManager) ResetChatWorkspace(string) (*meilisearch.ChatWorkspaceSettings, error)            { panic("stub") }
func (m *mockServiceManager) ResetChatWorkspaceWithContext(context.Context, string) (*meilisearch.ChatWorkspaceSettings, error) { panic("stub") }
func (m *mockServiceManager) AddWebhook(*meilisearch.AddWebhookRequest) (*meilisearch.Webhook, error)          { panic("stub") }
func (m *mockServiceManager) AddWebhookWithContext(context.Context, *meilisearch.AddWebhookRequest) (*meilisearch.Webhook, error) { panic("stub") }
func (m *mockServiceManager) UpdateWebhook(string, *meilisearch.UpdateWebhookRequest) (*meilisearch.Webhook, error) { panic("stub") }
func (m *mockServiceManager) UpdateWebhookWithContext(context.Context, string, *meilisearch.UpdateWebhookRequest) (*meilisearch.Webhook, error) { panic("stub") }
func (m *mockServiceManager) DeleteWebhook(string) error                                                        { panic("stub") }
func (m *mockServiceManager) DeleteWebhookWithContext(context.Context, string) error                            { panic("stub") }
func (m *mockServiceManager) ListWebhooks() (*meilisearch.WebhookResults, error)                                { panic("stub") }
func (m *mockServiceManager) ListWebhooksWithContext(context.Context) (*meilisearch.WebhookResults, error)      { panic("stub") }
func (m *mockServiceManager) GetWebhook(string) (*meilisearch.Webhook, error)                                   { panic("stub") }
func (m *mockServiceManager) GetWebhookWithContext(context.Context, string) (*meilisearch.Webhook, error)       { panic("stub") }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestClient creates a Client with the given mock ServiceManager injected.
func newTestClient(sm meilisearch.ServiceManager, logger *slog.Logger) *Client {
	return &Client{ms: sm, logger: logger}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestIndexer(sm *mockServiceManager) *Indexer {
	c := newTestClient(sm, testLogger())
	return NewIndexer(c, testLogger())
}
