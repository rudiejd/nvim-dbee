package clients

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kndndrj/nvim-dbee/dbee/core/builders"
	"github.com/kndndrj/nvim-dbee/dbee/core"
	_ "github.com/sijms/go-ora/v2"
)

// Register client
func init() {
	c := func(url string) (core.Client, error) {
		return NewOracle(url)
	}
	_ = register(c, "oracle")
}

var _ core.Client = (*Oracle)(nil)

type Oracle struct {
	c *builders.Client
}

func NewOracle(url string) (*Oracle, error) {
	db, err := sql.Open("oracle", url)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to oracle database: %v", err)
	}

	return &Oracle{
		c: builders.NewClient(db),
	}, nil
}

func (c *Oracle) Query(ctx context.Context, query string) (core.IterResult, error) {
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

	// Remove the trailing semicolon from the query - for some reason it isn't supported in go_ora
	query = strings.TrimSuffix(query, ";")

	// Use Exec or Query depending on the query
	action := strings.ToLower(strings.Split(query, " ")[0])
	hasReturnValues := strings.Contains(strings.ToLower(query), " returning ")
	if (action == "update" || action == "delete" || action == "insert") && !hasReturnValues {
		rows, err := con.Exec(ctx, query)
		if err != nil {
			return nil, err
		}
		rows.SetCallback(cb)
		return rows, nil
	}

	rows, err := con.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(rows.Header()) == 0 {
		rows.SetCustomHeader(core.Header{"No Results"})
	}
	rows.SetCallback(cb)

	return rows, nil
}

func (c *Oracle) Layout() ([]core.Layout, error) {
	query := `
		SELECT T.owner, T.table_name
		FROM (
			SELECT owner, table_name
			FROM all_tables
			UNION SELECT owner, view_name AS "table_name"
			FROM all_views
		) T
		JOIN all_users U ON T.owner = U.username
		WHERE U.common = 'NO'
		ORDER BY T.table_name
	`

	rows, err := c.Query(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	children := make(map[string][]core.Layout)

	for rows.HasNext() {
		row, err := rows.Next()
		if err != nil {
			return nil, err
		}

		// We know for a fact there are 2 string fields (see query above)
		schema := row[0].(string)
		table := row[1].(string)

		children[schema] = append(children[schema], core.Layout{
			Name:   table,
			Schema: schema,
			// TODO:
			Database: "",
			Type:     core.LayoutTypeTable,
		})

	}

	var layout []core.Layout

	for k, v := range children {
		layout = append(layout, core.Layout{
			Name:   k,
			Schema: k,
			// TODO:
			Database: "",
			Type:     core.LayoutTypeNone,
			Children: v,
		})
	}

	return layout, nil
}

func (c *Oracle) Close() {
	c.c.Close()
}
