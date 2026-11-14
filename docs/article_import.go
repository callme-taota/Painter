// Package docs embeds the structured article import format and authoring aids.
package docs

import _ "embed"

// ArticleImportSchema is the JSON Schema for the painter.article v1 envelope.
//
//go:embed painter-article.schema.json
var ArticleImportSchema string

// ArticleImportExample demonstrates the format using explicitly fictional data.
//
//go:embed examples/painter-article.json
var ArticleImportExample string

// ArticleImportPrompt is the copyable instruction for AI article conversion.
//
//go:embed prompts/painter-article-export.md
var ArticleImportPrompt string
