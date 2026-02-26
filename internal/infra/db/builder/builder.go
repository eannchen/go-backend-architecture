package builder

import sq "github.com/Masterminds/squirrel"

// StatementBuilder uses PostgreSQL-compatible placeholders ($1, $2, ...).
var StatementBuilder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
