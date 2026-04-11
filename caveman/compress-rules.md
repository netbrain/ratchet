# Caveman Compress Rules

Vendored from https://github.com/JuliusBrussee/caveman/caveman-compress (MIT License).
Used by `/ratchet:tighten` to compress pair definition files.

## Remove

- Articles: a, an, the
- Filler: just, really, basically, actually, simply, essentially, generally
- Pleasantries: "sure", "certainly", "of course", "happy to", "I'd recommend"
- Hedging: "it might be worth", "you could consider", "it would be good to"
- Redundant phrasing: "in order to" -> "to", "make sure to" -> "ensure", "the reason is because" -> "because"
- Connective fluff: "however", "furthermore", "additionally", "in addition"

## Preserve EXACTLY (never modify)

- Code blocks (fenced ``` and indented)
- Inline code (`backtick content`)
- URLs and links (full URLs, markdown links)
- File paths (`/src/components/...`, `./config.yaml`)
- Commands (`npm install`, `git commit`, `docker build`)
- Technical terms (library names, API names, protocols, algorithms)
- Proper nouns (project names, people, companies)
- Dates, version numbers, numeric values
- Environment variables (`$HOME`, `NODE_ENV`)

## Preserve Structure

- All markdown headings (keep exact heading text, compress body below)
- Bullet point hierarchy (keep nesting level)
- Numbered lists (keep numbering)
- Tables (compress cell text, keep structure)
- Frontmatter/YAML headers in markdown files

## Compress

- Use short synonyms: "big" not "extensive", "fix" not "implement a solution for", "use" not "utilize"
- Fragments OK: "Run tests before commit" not "You should always run tests before committing"
- Drop "you should", "make sure to", "remember to" -- just state the action
- Merge redundant bullets that say the same thing differently
- Keep one example where multiple examples show the same pattern

## Critical Rule

Anything inside ``` ... ``` must be copied EXACTLY. Do not remove comments, spacing, or reorder lines. Do not shorten commands or simplify anything.

Inline code (`...`) must be preserved EXACTLY. Do not modify anything inside backticks.

If file contains code blocks: treat code blocks as read-only regions. Only compress text outside them. Do not merge sections around code.

## Boundaries

- ONLY compress natural language files (.md, .txt)
- If file has mixed content (prose + code), compress ONLY the prose sections
- If unsure whether something is code or prose, leave it unchanged
- Original file is backed up as FILE.original.md before overwriting
- Never compress FILE.original.md (skip it)
