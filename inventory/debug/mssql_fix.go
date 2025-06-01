package debug

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"entgo.io/ent/dialect"
)

// quoteFixDriver is a wrapper around an ent dialect.Driver that converts
// identifier quoting from back-tick (`) style (used by MySQL) to square
// brackets ([]) that are accepted by Microsoft SQL Server. ent v0.13 does not
// emit the correct quoting for the "sqlserver" driver, resulting in queries
// that SQL Server rejects with the error: "Incorrect syntax near '`'".
//
// The wrapper is lightweight: It performs a cheap string transformation on the
// query *only* when the underlying driver reports Dialect() == "mssql".  For
// all other dialects the queries are passed through unchanged.
//
// Note: ent routes all queries through the driver layer (including inside
// transactions), therefore patching Exec / Query and their Context variants is
// sufficient to cover all code paths.

type quoteFixDriver struct {
	dialect.Driver
}

// newQuoteFixDriver wraps the given driver with the quoting fix. If the driver
// already targets a different dialect than MSSQL, it is returned unchanged.
func newQuoteFixDriver(d dialect.Driver) dialect.Driver {
	if d == nil {
		return d
	}
	dialectName := strings.ToLower(d.Dialect())
	if dialectName != "mssql" && dialectName != "sqlserver" {
		// Nothing to fix for other dialects.
		return d
	}
	return &quoteFixDriver{Driver: d}
}

// WrapMSSQLQuoteFix applies the quoting-fix wrapper on the given driver. It is
// a no-op for drivers of other dialects, returning the argument unchanged so
// that call-sites can remain agnostic of the underlying database.
func WrapMSSQLQuoteFix(d dialect.Driver) dialect.Driver {
	return newQuoteFixDriver(d)
}

// translate converts `identifier` into [identifier] quoting and
// replaces positional parameter placeholders (?) with SQL Server-style
// ordinal placeholders (@p1, @p2, …). The transformation skips over
// question marks that occur inside single-quoted string literals and
// correctly handles escaped quotes (").
func translate(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 16) // small extra buffer for @pN expansions

	openBracket := true // next back-tick becomes '[' when true, ']' when false
	inString := false   // inside single-quoted string literal
	paramIndex := 1     // current parameter counter for @pN

	for i := 0; i < len(query); i++ {
		c := query[i]

		switch c {
		case '\'':
			// Handle string literal boundaries / escaped quotes ('' inside string)
			b.WriteByte(c)
			if inString {
				// If next char is also a quote, it's an escaped quote – stay inside string.
				if i+1 < len(query) && query[i+1] == '\'' {
					// Write the next quote as well and advance index.
					b.WriteByte('\'')
					i++
					continue
				}
				inString = false
			} else {
				inString = true
			}

		case '`':
			if inString {
				// Inside string – leave untouched.
				b.WriteByte(c)
				continue
			}
			if openBracket {
				b.WriteByte('[')
			} else {
				b.WriteByte(']')
			}
			openBracket = !openBracket

		case '?':
			if inString {
				b.WriteByte(c)
				continue
			}
			// Replace with @pN
			b.WriteByte('@')
			b.WriteByte('p')
			b.WriteString(strconv.Itoa(paramIndex))
			paramIndex++

		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// maybeAddOutputClause injects "OUTPUT INSERTED.[id]" into INSERT statements so
// that SQL Server returns the auto-generated identity value. ent expects a row
// containing the primary key when it executes INSERTs via the Query path. The
// injection is conservative: it runs only if the statement is an INSERT and
// doesn't already contain an OUTPUT clause.
func maybeAddOutputClause(q string) string {
	lower := strings.ToLower(strings.TrimSpace(q))
	if !strings.HasPrefix(lower, "insert into") {
		return q
	}
	// Skip if caller already added an OUTPUT clause.
	if strings.Contains(lower, " output ") {
		return q
	}
	// Find the position of the "values" keyword (preceded by a space) to insert before it.
	idx := strings.Index(lower, " values")
	if idx == -1 {
		// Unexpected shape – leave unchanged.
		return q
	}
	// Insert OUTPUT clause right before "VALUES" (preserve original case of query slice).
	return q[:idx] + " OUTPUT INSERTED.[id]" + q[idx:]
}

// transformLimitQuery rewrites MySQL style "LIMIT" / "LIMIT .. OFFSET .." clauses
// into SQL-Server compatible TOP / OFFSET-FETCH forms. If the query ends with
// "LIMIT n" it converts it to "OFFSET 0 ROWS FETCH NEXT n ROWS ONLY" when an
// ORDER BY clause is present; otherwise it injects "TOP n" directly after the
// SELECT keyword. For "LIMIT n OFFSET m" it is always converted to OFFSET-FETCH
// (adding a dummy ORDER BY if the original query omitted one because SQL Server
// requires ORDER BY with OFFSET). The function is intentionally conservative –
// if it cannot confidently rewrite the query it returns the original string.
func transformLimitQuery(query string) string {
	trimmed := strings.TrimSpace(query)

	// Regexes for LIMIT syntax (case-insensitive)
	limitOnly := regexp.MustCompile(`(?i)\s+LIMIT\s+(\d+)\s*$`)
	limitOffset := regexp.MustCompile(`(?i)\s+LIMIT\s+(\d+)\s+OFFSET\s+(\d+)\s*$`)

	if m := limitOffset.FindStringSubmatch(trimmed); m != nil {
		limit, offset := m[1], m[2]
		base := strings.TrimSpace(trimmed[:len(trimmed)-len(m[0])])

		// Ensure we have an ORDER BY – add a dummy one if missing.
		lower := strings.ToLower(base)
		if !strings.Contains(lower, " order by ") {
			base += " ORDER BY (SELECT NULL)"
		}
		return base + " OFFSET " + offset + " ROWS FETCH NEXT " + limit + " ROWS ONLY"
	}

	if m := limitOnly.FindStringSubmatch(trimmed); m != nil {
		limit := m[1]
		base := strings.TrimSpace(trimmed[:len(trimmed)-len(m[0])])
		lower := strings.ToLower(base)

		if strings.Contains(lower, " order by ") {
			return base + " OFFSET 0 ROWS FETCH NEXT " + limit + " ROWS ONLY"
		}

		// Inject TOP n after SELECT for simple queries without ORDER BY.
		if idx := strings.Index(strings.ToLower(base), "select"); idx != -1 {
			insertPos := idx + len("select")
			return base[:insertPos] + " TOP " + limit + base[insertPos:]
		}
	}

	return query
}

// Exec implements dialect.Driver Exec with quote translation.
func (d *quoteFixDriver) Exec(ctx context.Context, query string, args, v any) error {
	return d.Driver.Exec(ctx, transformLimitQuery(translate(query)), args, v)
}

// ExecContext implements dialect.Driver ExecContext with quote translation.
func (d *quoteFixDriver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if drvCtx, ok := d.Driver.(interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	}); ok {
		return drvCtx.ExecContext(ctx, transformLimitQuery(translate(query)), args...)
	}
	// Fallback – driver does not implement the optional interface.
	var res sql.Result
	err := d.Driver.Exec(ctx, transformLimitQuery(translate(query)), args, &res)
	return res, err
}

// Query implements dialect.Driver Query with quote translation.
func (d *quoteFixDriver) Query(ctx context.Context, query string, args, v any) error {
	return d.Driver.Query(ctx, maybeAddOutputClause(transformLimitQuery(translate(query))), args, v)
}

// QueryContext implements dialect.Driver QueryContext with quote translation.
func (d *quoteFixDriver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	translated := maybeAddOutputClause(transformLimitQuery(translate(query)))
	if drvCtx, ok := d.Driver.(interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	}); ok {
		return drvCtx.QueryContext(ctx, translated, args...)
	}
	rows := &sql.Rows{}
	if err := d.Driver.Query(ctx, translated, args, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// Tx wraps the transaction returned from the underlying driver so that the
// quoting fix also applies inside transactions.
func (d *quoteFixDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	tx, err := d.Driver.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return &quoteFixTx{Tx: tx}, nil
}

// BeginTx wraps the transaction started by BeginTx (if supported by the
// underlying driver).
func (d *quoteFixDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (dialect.Tx, error) {
	if drvCtx, ok := d.Driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	}); ok {
		tx, err := drvCtx.BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &quoteFixTx{Tx: tx}, nil
	}
	return d.Tx(ctx)
}

type quoteFixTx struct {
	dialect.Tx
}

func (t *quoteFixTx) Exec(ctx context.Context, query string, args, v any) error {
	return t.Tx.Exec(ctx, transformLimitQuery(translate(query)), args, v)
}

func (t *quoteFixTx) Query(ctx context.Context, query string, args, v any) error {
	return t.Tx.Query(ctx, maybeAddOutputClause(transformLimitQuery(translate(query))), args, v)
}
