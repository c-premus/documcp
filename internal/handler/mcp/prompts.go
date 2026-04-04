package mcphandler

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerPrompts registers MCP prompt templates based on feature flags.
// Core prompts (document_analysis, search_query_builder, knowledge_base_builder,
// cross_source_research) are always registered. Source-specific prompts are
// registered only when their backing service is enabled.
func (h *Handler) registerPrompts(zimEnabled, gitTemplatesEnabled bool) {
	// Always registered
	h.server.AddPrompt(documentAnalysisPrompt(), h.handleDocumentAnalysis)
	h.server.AddPrompt(searchQueryBuilderPrompt(), h.handleSearchQueryBuilder)
	h.server.AddPrompt(knowledgeBaseBuilderPrompt(), h.handleKnowledgeBaseBuilder)
	h.server.AddPrompt(crossSourceResearchPrompt(), h.handleCrossSourceResearch)

	// Conditionally registered
	if gitTemplatesEnabled {
		h.server.AddPrompt(gitTemplateSetupPrompt(), h.handleGitTemplateSetup)
	}
	if zimEnabled {
		h.server.AddPrompt(zimResearchPrompt(), h.handleZimResearch)
	}
}

// --- Prompt definitions ---

func documentAnalysisPrompt() *mcp.Prompt {
	return &mcp.Prompt{
		Name:        "document_analysis",
		Description: "Analyze one or more documents with configurable task, focus area, and output length",
		Arguments: []*mcp.PromptArgument{
			{Name: "document_ids", Description: "Comma-separated UUIDs of documents to analyze", Required: true},
			{Name: "task", Description: "Analysis task: summarize, compare, extract, assess (default: summarize)"},
			{Name: "focus", Description: "Focus area: technical, business, overview, actionable (default: overview)"},
			{Name: "length", Description: "Output length: brief, detailed, comprehensive (default: detailed)"},
		},
	}
}

func searchQueryBuilderPrompt() *mcp.Prompt {
	return &mcp.Prompt{
		Name:        "search_query_builder",
		Description: "Build optimized search queries for the knowledge base given a research goal",
		Arguments: []*mcp.PromptArgument{
			{Name: "goal", Description: "What you are trying to find or learn", Required: true},
			{Name: "context", Description: "Additional context to narrow the search (e.g., project name, technology)"},
			{Name: "file_types", Description: "Preferred file types to search (e.g., markdown, pdf, docx)"},
		},
	}
}

func knowledgeBaseBuilderPrompt() *mcp.Prompt {
	return &mcp.Prompt{
		Name:        "knowledge_base_builder",
		Description: "Create or expand knowledge base content with structured documents",
		Arguments: []*mcp.PromptArgument{
			{Name: "goal", Description: "What the new content should cover", Required: true},
			{Name: "content_type", Description: "Content type: guide, runbook, reference, tutorial, adr (default: guide)"},
			{Name: "scope", Description: "Scope: single document or multi-document series (default: single)"},
		},
	}
}

func gitTemplateSetupPrompt() *mcp.Prompt {
	return &mcp.Prompt{
		Name:        "git_template_setup",
		Description: "Set up a project from git templates with guided variable substitution and deployment",
		Arguments: []*mcp.PromptArgument{
			{Name: "intent", Description: "What you want to set up (e.g., new Go service, CLAUDE.md, Memory Bank)", Required: true},
			{Name: "category", Description: "Template category filter (e.g., claude, memory-bank, service)"},
			{Name: "depth", Description: "Detail level: browse, configure, deploy (default: deploy)"},
		},
	}
}

func zimResearchPrompt() *mcp.Prompt {
	return &mcp.Prompt{
		Name:        "zim_research",
		Description: "Research a topic using offline ZIM archives (DevDocs, Wikipedia, Stack Exchange)",
		Arguments: []*mcp.PromptArgument{
			{Name: "topic", Description: "The topic to research", Required: true},
			{Name: "depth", Description: "Research depth: quick, standard, deep (default: standard)"},
			{Name: "preferred_sources", Description: "Preferred ZIM archives (e.g., devdocs, wikipedia, stackexchange)"},
		},
	}
}

func crossSourceResearchPrompt() *mcp.Prompt {
	return &mcp.Prompt{
		Name:        "cross_source_research",
		Description: "Research a topic across all available sources (documents, ZIM archives, templates)",
		Arguments: []*mcp.PromptArgument{
			{Name: "topic", Description: "The topic to research across sources", Required: true},
			{Name: "sources", Description: "Comma-separated source list: documents, zim, templates (default: all available)"},
			{Name: "depth", Description: "Research depth: quick, standard, deep (default: standard)"},
		},
	}
}

// --- Prompt handlers ---

func (h *Handler) handleDocumentAnalysis(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments

	documentIDs := args["document_ids"]
	task := defaultArg(args, "task", "summarize")
	focus := defaultArg(args, "focus", "overview")
	length := defaultArg(args, "length", "detailed")

	taskGuidance := documentTaskGuidance(task)
	focusGuidance := documentFocusGuidance(focus)
	lengthGuidance := documentLengthGuidance(length)

	assistantText := fmt.Sprintf(`You are a document analysis assistant with access to the DocuMCP knowledge base.

**Available tool:** read_document (retrieve document content by UUID)

**Your task:** %s
**Focus area:** %s
**Output length:** %s

## Workflow

1. Read each document using read_document with the provided UUIDs.
2. Analyze the content according to the task and focus area.
3. Produce output matching the requested length.

## Task guidance
%s

## Focus guidance
%s

## Length guidance
%s

## Output format

Structure your response with clear headings. Use bullet points for key findings and code blocks for technical content. End with a summary of actionable insights when relevant.`,
		task, focus, length,
		taskGuidance, focusGuidance, lengthGuidance,
	)

	userText := fmt.Sprintf(`Please analyze the following documents:

**Document UUIDs:** %s

**Task:** %s
**Focus:** %s
**Length:** %s

Read each document using read_document, then provide your analysis following the guidelines above.`, documentIDs, task, focus, length)

	return &mcp.GetPromptResult{
		Description: "Document analysis prompt for: " + task,
		Messages: []*mcp.PromptMessage{
			{Role: "assistant", Content: &mcp.TextContent{Text: assistantText}},
			{Role: "user", Content: &mcp.TextContent{Text: userText}},
		},
	}, nil
}

func (h *Handler) handleSearchQueryBuilder(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments

	goal := args["goal"]
	searchContext := args["context"]
	fileTypes := args["file_types"]

	var contextSection string
	if searchContext != "" {
		contextSection = "\n**Additional context:** " + searchContext
	}

	var fileTypeSection string
	if fileTypes != "" {
		fileTypeSection = "\n**Preferred file types:** " + fileTypes
	}

	assistantText := `You are a search query optimization assistant with access to the DocuMCP knowledge base.

**Available tools:**
- unified_search: Search across ALL sources (documents, ZIM archives, git templates, ZIM article content) in one request. Supports a ` + "`types`" + ` filter to narrow by source.
- search_documents: Full-text search within uploaded documents with filters for file type, tags, and pagination.

## Query optimization tips

1. **Start broad, then narrow.** Use unified_search first for cross-source discovery, then search_documents for targeted results.
2. **Use key terms, not full sentences.** Extract the most meaningful 2-4 words.
3. **Try synonyms and related terms.** Technical concepts often have multiple names.
4. **Leverage filters.** Use file_type and tag filters on search_documents to reduce noise. Use the types filter on unified_search to limit sources.
5. **Iterate.** Review initial results, then refine based on what you find.

## Search strategy

- Start with unified_search for a broad overview across all sources.
- Follow up with search_documents for document-specific deep dives using file_type and tag filters.
- Run 2-3 queries with different term combinations.
- Review result snippets before reading full documents.
- Summarize findings and suggest follow-up queries if needed.

## Example tool calls

` + "```" + `
unified_search(query: "kubernetes deployment strategy", limit: 10)
search_documents(query: "kubernetes deployment strategy", file_type: "markdown")
search_documents(query: "k8s rollout canary blue-green", tags: ["devops", "infrastructure"])
` + "```"

	userText := fmt.Sprintf(`I need help finding information in the knowledge base.

**Search goal:** %s%s%s

Please build optimized search queries for this goal. Run the searches using search_documents, review the results, and provide a summary of what you found along with any recommended follow-up queries.`,
		goal, contextSection, fileTypeSection,
	)

	return &mcp.GetPromptResult{
		Description: "Search query builder for: " + truncate(goal, 60),
		Messages: []*mcp.PromptMessage{
			{Role: "assistant", Content: &mcp.TextContent{Text: assistantText}},
			{Role: "user", Content: &mcp.TextContent{Text: userText}},
		},
	}, nil
}

func (h *Handler) handleKnowledgeBaseBuilder(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments

	goal := args["goal"]
	contentType := defaultArg(args, "content_type", "guide")
	scope := defaultArg(args, "scope", "single")

	templateGuidance := contentTypeTemplate(contentType)

	assistantText := fmt.Sprintf(`You are a knowledge base content builder with access to the DocuMCP knowledge base.

**Available tools:**
- create_document: Create new documents (markdown or html). Content is auto-indexed for search.
- update_document: Modify existing document metadata (title, description, tags, visibility)
- search_documents: Search existing content to avoid duplication and find related documents

**Content type:** %s
**Scope:** %s

## Workflow

1. **Research first.** Search for existing content on the topic to avoid duplication and find related material.
2. **Plan the structure.** Outline sections based on the content type template below.
3. **Write the content.** Create well-structured, searchable documents with clear titles and descriptions.
4. **Tag appropriately.** Add relevant tags to make the content discoverable.

## Content type template: %s
%s

## Scope guidance

%s

## Best practices

- Use descriptive titles that work well in search results.
- Write a clear description (1-2 sentences) for each document.
- Add 3-5 relevant tags per document.
- Use markdown formatting for readability.
- Include cross-references to related documents when applicable.`,
		contentType, scope, contentType, templateGuidance,
		scopeGuidance(scope),
	)

	userText := fmt.Sprintf(`I want to create new knowledge base content.

**Goal:** %s
**Content type:** %s
**Scope:** %s

Please search for existing related content first, then create the new documents following the content type template and best practices above.`,
		goal, contentType, scope,
	)

	return &mcp.GetPromptResult{
		Description: "Knowledge base builder for: " + truncate(goal, 60),
		Messages: []*mcp.PromptMessage{
			{Role: "assistant", Content: &mcp.TextContent{Text: assistantText}},
			{Role: "user", Content: &mcp.TextContent{Text: userText}},
		},
	}, nil
}

func (h *Handler) handleGitTemplateSetup(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments

	intent := args["intent"]
	category := args["category"]
	depth := defaultArg(args, "depth", "deploy")

	var categoryFilter string
	if category != "" {
		categoryFilter = "\n**Category filter:** " + category
	}

	assistantText := fmt.Sprintf(`You are a project setup assistant with access to git template tools.

**Available tools:**
- list_git_templates: List available templates with optional category filter
- search_git_templates: Full-text search across template files, README content, and metadata. Returns matched_files paths.
- get_template_structure: View folder tree, essential files, and required variables
- get_template_file: Retrieve individual file content with variable substitution
- get_deployment_guide: Get deployment instructions with all essential files
- download_template: Download a complete template as a base64-encoded archive

**Depth level:** %s

## Workflow

### Step 1: Discover
List or search templates to find the best match for the user's intent.

`+"```"+`
list_git_templates(category: "claude")
search_git_templates(query: "memory bank setup")
`+"```"+`

### Step 2: Inspect
Review the template structure and required variables.

`+"```"+`
get_template_structure(uuid: "...")
`+"```"+`

### Step 3: Configure
Retrieve key files and help the user fill in required variables.

`+"```"+`
get_template_file(uuid: "...", path: "CLAUDE.md", variables: "{\"project_name\": \"my-app\"}")
`+"```"+`

### Step 4: Deploy
Provide the full deployment guide or download the template archive.

`+"```"+`
get_deployment_guide(uuid: "...")
download_template(uuid: "...")
`+"```"+`

## Depth guidance

- **browse**: Steps 1-2 only. List templates and show structure.
- **configure**: Steps 1-3. Inspect files and resolve variables.
- **deploy**: All steps. Full setup including deployment instructions.`,
		depth,
	)

	userText := fmt.Sprintf(`I want to set up a project using git templates.

**Intent:** %s%s
**Depth:** %s

Please find the right template and guide me through the setup process at the requested depth level.`,
		intent, categoryFilter, depth,
	)

	return &mcp.GetPromptResult{
		Description: "Git template setup for: " + truncate(intent, 60),
		Messages: []*mcp.PromptMessage{
			{Role: "assistant", Content: &mcp.TextContent{Text: assistantText}},
			{Role: "user", Content: &mcp.TextContent{Text: userText}},
		},
	}, nil
}

func (h *Handler) handleZimResearch(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments

	topic := args["topic"]
	depth := defaultArg(args, "depth", "standard")
	preferredSources := args["preferred_sources"]

	var sourcesSection string
	if preferredSources != "" {
		sourcesSection = "\n**Preferred sources:** " + preferredSources
	}

	depthGuidance := zimDepthGuidance(depth)

	assistantText := fmt.Sprintf(`You are a research assistant with access to offline ZIM archives (DevDocs, Wikipedia, Stack Exchange, and more).

**Available tools:**
- list_zim_archives: List available archives with category and language filters
- search_zim: Search within an archive (modes: "suggest" for title matching, "fulltext" for content search)
- read_zim_article: Retrieve article content (supports summary_only and max_paragraphs)

**Research depth:** %s

## 4-step research workflow

### Step 1: Discover archives
List available ZIM archives to identify relevant sources.

`+"```"+`
list_zim_archives(category: "stack_exchange")
list_zim_archives(category: "devdocs")
`+"```"+`

### Step 2: Search for content
Use "fulltext" (default) for content searches, "suggest" for fast title lookups.

`+"```"+`
search_zim(archive: "devdocs_en_laravel", query: "topic", search_type: "fulltext")
search_zim(archive: "wikipedia_en", query: "topic", search_type: "suggest")
`+"```"+`

### Step 3: Read and analyze
Retrieve the most relevant articles. Use summary_only for quick scans.

`+"```"+`
read_zim_article(archive: "devdocs_en_laravel", path: "/article", summary_only: true)
read_zim_article(archive: "wikipedia_en", path: "/article", max_paragraphs: 10)
`+"```"+`

### Step 4: Synthesize
Combine findings from multiple sources into a coherent summary.

## Depth guidance
%s`, depth, depthGuidance)

	userText := fmt.Sprintf(`I want to research a topic using the ZIM archives.

**Topic:** %s
**Depth:** %s%s

Please follow the 4-step research workflow to find, read, and synthesize information about this topic.`,
		topic, depth, sourcesSection,
	)

	return &mcp.GetPromptResult{
		Description: "ZIM research for: " + truncate(topic, 60),
		Messages: []*mcp.PromptMessage{
			{Role: "assistant", Content: &mcp.TextContent{Text: assistantText}},
			{Role: "user", Content: &mcp.TextContent{Text: userText}},
		},
	}, nil
}

func (h *Handler) handleCrossSourceResearch(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments

	topic := args["topic"]
	sources := args["sources"]
	depth := defaultArg(args, "depth", "standard")

	if sources == "" {
		sources = "all available"
	}

	depthGuidance := crossSourceDepthGuidance(depth)

	assistantText := fmt.Sprintf(`You are a cross-source research assistant with access to multiple knowledge sources.

**Available tool groups:**

**Unified Search (always available):**
- unified_search: Search across ALL sources in one request. Use the `+"`types`"+` filter to narrow: document, git_template, zim_archive, zim_article.

**Documents (always available):**
- search_documents: Full-text search with file type and tag filters
- read_document: Retrieve document content by UUID

**ZIM Archives (if enabled):**
- list_zim_archives: List offline archives (DevDocs, Wikipedia, Stack Exchange)
- search_zim: Search within a specific archive (archive, query, search_type: fulltext or suggest)
- read_zim_article: Retrieve article content (archive, path, summary_only, max_paragraphs)

**Git Templates (if enabled):**
- list_git_templates: List project templates
- search_git_templates: Search template files, README content, and metadata
- get_template_structure: View template folder tree and variables (uuid)

**Research depth:** %s
**Sources to search:** %s

## Cross-source research workflow

### Step 1: Broad discovery
Start with unified_search to get an overview across all sources. Use the types filter to focus.

`+"```"+`
unified_search(query: "topic keywords", limit: 10)
unified_search(query: "topic keywords", types: ["document", "zim_article"], limit: 10)
`+"```"+`

### Step 2: Source-specific deep dives
Use source-specific tools for detailed searches in promising sources.

`+"```"+`
search_documents(query: "topic", file_type: "markdown")
search_zim(archive: "devdocs_en_laravel", query: "topic", search_type: "fulltext")
`+"```"+`

### Step 3: Read and collect
Retrieve full content from the most relevant results in each source.

`+"```"+`
read_document(uuid: "...")
read_zim_article(archive: "...", path: "/article")
`+"```"+`

### Step 4: Cross-reference and synthesize
Compare information across sources, noting agreements, contradictions, and unique insights from each source.

## Depth guidance
%s

## Output format

Structure your final summary as:
1. **Overview**: High-level findings across all sources.
2. **By source**: Key findings from each source searched.
3. **Cross-references**: Where sources agree, disagree, or complement each other.
4. **Gaps**: Topics where information was missing or incomplete.
5. **Recommendations**: Suggested next steps or follow-up queries.`,
		depth, sources, depthGuidance,
	)

	userText := fmt.Sprintf(`I want to research a topic across multiple knowledge sources.

**Topic:** %s
**Sources:** %s
**Depth:** %s

Please search across the specified sources, read the most relevant content, and provide a cross-referenced synthesis of your findings.`,
		topic, sources, depth,
	)

	return &mcp.GetPromptResult{
		Description: "Cross-source research for: " + truncate(topic, 60),
		Messages: []*mcp.PromptMessage{
			{Role: "assistant", Content: &mcp.TextContent{Text: assistantText}},
			{Role: "user", Content: &mcp.TextContent{Text: userText}},
		},
	}, nil
}

// --- Helpers ---

// defaultArg returns the argument value for the given key, or the fallback if
// the key is absent or empty.

// --- Task/focus/length guidance for document_analysis ---

func documentTaskGuidance(task string) string {
	switch strings.ToLower(task) {
	case "compare":
		return `Compare the documents side by side. Identify similarities, differences, and contradictions. Produce a comparison matrix or structured diff.`
	case "extract":
		return `Extract specific data points, facts, or structured information from the documents. Present findings in a table or structured list.`
	case "assess":
		return `Assess the documents for quality, completeness, accuracy, and relevance. Provide ratings or scores where appropriate with justification.`
	default: // summarize
		return `Summarize each document's key points and themes. Highlight the most important information and conclusions.`
	}
}

func documentFocusGuidance(focus string) string {
	switch strings.ToLower(focus) {
	case "technical":
		return `Emphasize technical details: architecture, APIs, data models, algorithms, and implementation specifics. Use precise terminology.`
	case "business":
		return `Emphasize business impact: costs, timelines, risks, stakeholders, and strategic implications. Use accessible language.`
	case "actionable":
		return `Focus on what to do next: action items, decisions needed, blockers, and recommendations. Prioritize by urgency and impact.`
	default: // overview
		return `Provide a balanced overview covering all aspects of the documents. Suitable for a general audience.`
	}
}

func documentLengthGuidance(length string) string {
	switch strings.ToLower(length) {
	case "brief":
		return `Keep the output concise: 2-4 paragraphs or a short bullet list. Focus on the single most important takeaway per document.`
	case "comprehensive":
		return `Provide exhaustive analysis covering every section and detail. Include direct quotes and specific references. No length limit.`
	default: // detailed
		return `Provide a thorough analysis with clear structure. Cover all major points but avoid unnecessary repetition. Aim for a well-organized document of moderate length.`
	}
}

// --- Content type templates for knowledge_base_builder ---

func contentTypeTemplate(contentType string) string {
	switch strings.ToLower(contentType) {
	case "runbook":
		return `
- **Title**: Clear action-oriented title (e.g., "Runbook: Restart Production Database")
- **Sections**: Trigger / Prerequisites / Steps / Verification / Rollback / Escalation
- **Style**: Numbered steps, copy-pasteable commands, explicit expected outputs`
	case "reference":
		return `
- **Title**: Descriptive reference title (e.g., "API Reference: User Service v2")
- **Sections**: Overview / Endpoints or Components / Parameters / Examples / Error Codes
- **Style**: Tables for parameters, code blocks for examples, alphabetical ordering`
	case "tutorial":
		return `
- **Title**: Goal-oriented title (e.g., "Tutorial: Build a REST API with Go and Chi")
- **Sections**: Prerequisites / Introduction / Step-by-step instructions / Summary / Next steps
- **Style**: Progressive complexity, working code at each step, explanations of why not just how`
	case "adr":
		return `
- **Title**: "ADR-NNN: Decision Title" (e.g., "ADR-001: Use PostgreSQL for Primary Storage")
- **Sections**: Status / Context / Decision / Consequences / Alternatives Considered
- **Style**: Concise, factual, neutral tone. Focus on trade-offs and rationale.`
	default: // guide
		return `
- **Title**: Descriptive guide title (e.g., "Guide: Setting Up Local Development Environment")
- **Sections**: Overview / Prerequisites / Instructions / Troubleshooting / References
- **Style**: Clear prose, diagrams where helpful, links to related guides`
	}
}

func scopeGuidance(scope string) string {
	switch strings.ToLower(scope) {
	case "multi":
		return `Create a series of related documents. Plan the series structure first, then create each document with cross-references. Use consistent naming and tagging conventions across the series.`
	default: // single
		return `Create a single, self-contained document. Keep it focused on one topic. Reference existing documents rather than duplicating content.`
	}
}

// --- Depth guidance for research prompts ---

func zimDepthGuidance(depth string) string {
	switch strings.ToLower(depth) {
	case "quick":
		return `Search 1-2 archives. Read article summaries only (summary_only: true). Provide a brief answer with source references.`
	case "deep":
		return `Search all relevant archives. Read full articles (no max_paragraphs limit). Cross-reference multiple sources. Provide an exhaustive synthesis with detailed citations.`
	default: // standard
		return `Search 2-3 archives. Read key articles with max_paragraphs: 15. Provide a thorough summary covering the main aspects of the topic.`
	}
}

func crossSourceDepthGuidance(depth string) string {
	switch strings.ToLower(depth) {
	case "quick":
		return `Use unified_search for a quick overview. Read 2-3 top results from the most relevant source. Provide a brief summary.`
	case "deep":
		return `Search every available source thoroughly. Read 5+ items per source. Perform multiple search iterations with refined queries. Provide an exhaustive cross-referenced synthesis.`
	default: // standard
		return `Use unified_search first, then do targeted searches in 2-3 sources. Read 3-5 items per source. Provide a structured cross-source summary.`
	}
}
