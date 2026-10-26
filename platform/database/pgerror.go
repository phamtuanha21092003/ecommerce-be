package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	uniqueViolationCode     = "23505"
	foreignKeyViolationCode = "23503"
	checkViolationCode      = "23514"
)

func pgError(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil, false
	}
	return pgErr, true
}

// IsUniqueViolation reports a unique constraint failure.
// An empty constraint matches any unique violation.
func IsUniqueViolation(err error, constraint string) bool {
	pgErr, ok := pgError(err)
	if !ok || pgErr.Code != uniqueViolationCode {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// IsForeignKeyViolation reports a foreign key constraint failure.
func IsForeignKeyViolation(err error) bool {
	pgErr, ok := pgError(err)
	return ok && pgErr.Code == foreignKeyViolationCode
}

// IsCheckViolation reports a CHECK constraint failure.
func IsCheckViolation(err error) bool {
	pgErr, ok := pgError(err)
	return ok && pgErr.Code == checkViolationCode
}
