package commands

import "fmt"

// Wiki prompt templates for the /wiki slash command system.
// Based on Karpathy's LLM Wiki pattern, adapted from DeepVcodeClient's implementation.

const (
	wikiDir       = ".llm-wiki"
	wikiRawDir    = ".llm-wiki/raw"
	wikiPagesDir  = ".llm-wiki/wiki"
	wikiIndexPath = ".llm-wiki/index.md"
	wikiLogPath   = ".llm-wiki/log.md"
)

// WikiInitPrompt is the prompt for `/wiki init`.
const WikiInitPrompt = `
You are a knowledge base maintainer following the LLM Wiki pattern.

**Task: Initialize the wiki directory structure in .llm-wiki/.**

1. Create directory: .llm-wiki/raw/ (for immutable source documents)
2. Create directory: .llm-wiki/wiki/ (for AI-maintained knowledge pages)
3. Create .llm-wiki/index.md with this structure:
   - Title: "# LLM Wiki Index"
   - Four sections: ## Sources, ## Entities, ## Concepts, ## Synthesis
   - Brief description of each section
4. Create .llm-wiki/log.md with initial entry:
   - Title: "# LLM Wiki Log"
   - Entry: "## [YYYY-MM-DD] init | Wiki Initialized"
5. Create .llm-wiki/wiki/overview.md:
   - YAML frontmatter: type: entity, date, tags
   - Infer project info from README, go.mod, main entry points
   - Brief architecture overview, tech stack, key directories

**Rules:**
- Use YAML frontmatter on all pages (type, date, tags)
- Use [[wikilinks]] for cross-references between pages
- Never modify files in .llm-wiki/raw/ — those are immutable sources
- After creating all files, confirm the structure is ready
`

// GetWikiIngestPrompt builds the prompt for `/wiki ingest <path>`.
// If path is empty, it ingests all un-ingested files in raw/.
func GetWikiIngestPrompt(sourcePath string) string {
	if sourcePath != "" {
		return fmt.Sprintf(
			"\n"+
				"You are a knowledge base maintainer. The wiki lives in .llm-wiki/.\n\n"+
				"**Task: Ingest a source document into the wiki.**\n\n"+
				"Source to ingest: `%s`\n\n"+
				"**Workflow:**\n\n"+
				"1. **Read the source** — Read the file at `%s` completely.\n"+
				"2. **Read the wiki index** — Read .llm-wiki/index.md to understand existing pages.\n"+
				"3. **Extract** — Identify key entities, concepts, facts, claims, and relationships.\n"+
				"4. **Create source summary** — Write a page in .llm-wiki/wiki/ named after the source:\n"+
				"   - YAML frontmatter: type: source, source_path, date, tags, related\n"+
				"   - Key takeaways\n"+
				"   - Important entities and concepts\n"+
				"   - Notable code references\n"+
				"   - Contradictions with existing wiki pages\n"+
				"5. **Update entity/concept pages** — For each significant entity or concept:\n"+
				"   - If a page exists in .llm-wiki/wiki/, update it with new info from this source\n"+
				"   - If no page exists, create one with YAML frontmatter (type: entity or type: concept)\n"+
				"   - Use [[wikilinks]] for cross-references\n"+
				"6. **Update index.md** — Add new source summary and any new pages to appropriate sections.\n"+
				"7. **Append to log.md** — Add entry: \"## [YYYY-MM-DD] ingest | {source name}\"\n"+
				"   List which pages were created or updated.\n\n"+
				"**Rules:**\n"+
				"- Always use YAML frontmatter (type, date, tags)\n"+
				"- Use [[wikilinks]] for cross-references between wiki pages\n"+
				"- Never modify files in .llm-wiki/raw/ — those are immutable sources\n"+
				"- Flag any contradictions with existing wiki content explicitly\n"+
				"- A single ingest may touch 5-15 wiki pages — that's normal\n",
			sourcePath, sourcePath)
	}
	return `
You are a knowledge base maintainer. The wiki lives in .llm-wiki/.

**Task: Ingest ALL source documents in .llm-wiki/raw/ that have not yet been ingested.**

**Workflow:**

1. **List raw sources** — List all files in .llm-wiki/raw/ (recursively).
2. **Check existing** — Read .llm-wiki/index.md to see which sources are already ingested.
3. **For each un-ingested source**, run the full ingest workflow:
   a. Read the source file completely
   b. Read .llm-wiki/index.md to understand existing pages
   c. Identify key entities, concepts, facts, relationships
   d. Create source summary page in .llm-wiki/wiki/ with YAML frontmatter
   e. Update or create entity/concept pages with [[wikilinks]]
   f. Update .llm-wiki/index.md with new entries
   g. Append to .llm-wiki/log.md
4. **Summary** — After all sources processed, report what was ingested.

**Rules:**
- Process sources one by one, do not skip any
- Use [[wikilinks]] for cross-references
- Always include YAML frontmatter (type, date, tags)
- Never modify files in .llm-wiki/raw/
- Flag contradictions between sources
- Skip sources that already have a corresponding summary page
`
}

// GetWikiQueryPrompt builds the prompt for `/wiki query <question>`.
func GetWikiQueryPrompt(question string) string {
	return fmt.Sprintf(
		"\n"+
			"You are a knowledge base assistant. The wiki lives in .llm-wiki/.\n\n"+
			"**Task: Answer a question using the wiki.**\n\n"+
			"Question: %s\n\n"+
			"**Workflow:**\n\n"+
			"1. **Read index** — Start by reading .llm-wiki/index.md to find relevant pages.\n"+
			"2. **Read pages** — Read the wiki pages in .llm-wiki/wiki/ most relevant to the question.\n"+
			"3. **Synthesize** — Provide a comprehensive answer with citations to specific wiki pages.\n"+
			"4. **Suggest** — If answer contains valuable analysis, ask if user wants to save as a new wiki page.\n\n"+
			"**Rules:**\n"+
			"- **ONLY read from .llm-wiki/wiki/ pages. NEVER read from .llm-wiki/raw/ directly.**\n"+
			"  The raw directory contains unprocessed source documents — if wiki pages don't\n"+
			"  have the information, tell the user to run `/wiki ingest <file>` on the\n"+
			"  relevant raw sources first.\n"+
			"- Always cite which wiki pages informed your answer\n"+
			"- If wiki doesn't contain enough info, say so and suggest which raw sources to ingest\n"+
			"- Highlight contradictions between wiki pages if found\n",
		question)
}

// WikiLintPrompt is the prompt for `/wiki lint`.
const WikiLintPrompt = `
You are a knowledge base health checker. The wiki lives in .llm-wiki/.

**Task: Perform a health check on the wiki.**

**Workflow:**

1. **Read index** — Read .llm-wiki/index.md for the full page catalog.
2. **Scan pages** — Scan all pages in .llm-wiki/wiki/.
3. **Check for issues:**
   - **Orphan pages**: Pages in .llm-wiki/wiki/ but not listed in .llm-wiki/index.md
   - **Dead links**: [[wikilinks]] pointing to non-existent pages
   - **Missing pages**: Entities/concepts frequently mentioned but lacking their own page
   - **Stale content**: Pages with old dates in frontmatter
   - **Contradictions**: Claims that conflict across different pages
   - **Missing cross-references**: Pages that should link to each other but don't
   - **Incomplete frontmatter**: Pages missing type, date, or tags
4. **Report** — List findings with specific file paths and line references.
5. **Fix simple issues** — Update index, add cross-references, fix frontmatter.
   Ask before making larger changes.
6. **Append to log.md** — "## [YYYY-MM-DD] lint | Health Check" with summary.
`
