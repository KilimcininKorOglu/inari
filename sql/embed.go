// Package sql embeds the SQLite schema file for Inari's graph database.
package sql

import _ "embed"

// SchemaSQL contains the complete SQLite schema for the Inari graph database.
// This is the single source of truth for the database structure.
//
//go:embed schema.sql
var SchemaSQL string
