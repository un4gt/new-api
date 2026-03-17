package common

const (
	DatabaseTypePostgreSQL = "postgres"
)

// Database flags
//
// This minimal build is PostgreSQL-only. Other database backends are not
// supported and related flags are intentionally kept as constant false to
// avoid scattered conditional logic across the codebase.
var UsingPostgreSQL = false
var UsingSQLite = false
var UsingMySQL = false

// LogSqlType indicates which database backend is used for log SQL queries.
// With PostgreSQL-only support, this should always remain "postgres" once DB is initialized.
var LogSqlType = DatabaseTypePostgreSQL
