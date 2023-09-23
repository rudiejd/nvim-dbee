//go:build (darwin && (amd64 || arm64)) || (freebsd && (386 || amd64 || arm || arm64)) || (linux && (386 || amd64 || arm || arm64 || ppc64le || riscv64 || s390x)) || (netbsd && amd64) || (openbsd && (amd64 || arm64)) || (windows && (amd64 || arm64))

package clients

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kndndrj/nvim-dbee/dbee/core/builders"
	"github.com/kndndrj/nvim-dbee/dbee/core"
	_ "modernc.org/sqlite"
)

// Register client
func init() {
	c := func(url string) (core.Client, error) {
		return NewSqlite(url)
	}
	_ = register(c, "sqlite", "sqlite3")
}

var _ core.Client = (*SQLite)(nil)

type SQLite struct {
	c *builders.Client
}

func NewSqlite(url string) (*SQLite, error) {
	db, err := sql.Open("sqlite", url)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to sqlite database: %v", err)
	}

	return &SQLite{
		c: builders.NewClient(db),
	}, nil
}

func (c *SQLite) Query(ctx context.Context, query string) (core.IterResult, error) {
	con, err := c.c.Conn(ctx)
	if err != nil {
		return nil, err
	}
	cb := func() {
		con.Close()
	}
	defer func() {
		if err != nil {
			cb()
		}
	}()

	rows, err := con.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(rows.Header()) > 0 {
		rows.SetCallback(cb)
		return rows, nil
	}
	rows.Close()

	// empty header means no result -> get affected rows
	rows, err = con.Query(ctx, "select changes() as 'Rows Affected'")
	rows.SetCallback(cb)
	return rows, err
}

func (c *SQLite) Layout() ([]core.Layout, error) {
	query := `SELECT name FROM sqlite_schema WHERE type ='table'`

	rows, err := c.Query(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	var schema []core.Layout
	for rows.HasNext() {
		row, err := rows.Next()
		if err != nil {
			return nil, err
		}

		// We know for a fact there is only one string field (see query above)
		table := row[0].(string)
		schema = append(schema, core.Layout{
			Name:   table,
			Schema: "",
			// TODO:
			Database: "",
			Type:     core.LayoutTypeTable,
		})
	}

	return schema, nil
}

func (c *SQLite) Close() {
	c.c.Close()
}
