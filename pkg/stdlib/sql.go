/*
Copyright 2021 CodeNotary, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package stdlib

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"github.com/codenotary/immudb/pkg/api/schema"
	"github.com/codenotary/immudb/pkg/client"
	"google.golang.org/grpc/metadata"
	"io"
	"math"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

var immuDriver *Driver

func init() {
	immuDriver = &Driver{
		configs: make(map[string]*Conn),
	}
	sql.Register("immudb", immuDriver)
}

type DBOption func(*Connector)

type ConnConfig struct {
	client.Options
}

func OpenDB(cliOpts *client.Options) *sql.DB {
	c := &Connector{
		cliOptions: cliOpts,
		driver:     immuDriver,
	}

	return sql.OpenDB(c)
}

type Connector struct {
	cliOptions *client.Options
	driver     *Driver
}

// Connect implement driver.Connector interface
func (c Connector) Connect(ctx context.Context) (driver.Conn, error) {
	name, err := GetUri(c.cliOptions)
	if err != nil {
		return nil, err
	}
	c.driver.configMutex.Lock()
	cn := c.driver.configs[name]
	c.driver.configMutex.Unlock()

	if cn == nil {
		if c.cliOptions == nil {
			var err error
			c.cliOptions, err = ParseConfig(name)
			if err != nil {
				return nil, err
			}
		}
		conn, err := client.NewImmuClient(c.cliOptions)
		if err != nil {
			return nil, err
		}
		lr, err := conn.Login(ctx, []byte(c.cliOptions.Username), []byte(c.cliOptions.Password))
		if err != nil {
			return nil, err
		}

		md := metadata.Pairs("authorization", lr.Token)
		ctx = metadata.NewOutgoingContext(ctx, md)

		resp, err := conn.UseDatabase(ctx, &schema.Database{DatabaseName: c.cliOptions.Database})
		if err != nil {
			return nil, err
		}

		cn = &Conn{
			conn:    conn,
			options: c.cliOptions,
			Token:   resp.Token,
		}
	}

	c.driver.configMutex.Lock()
	c.driver.configs[name] = cn
	c.driver.configMutex.Unlock()

	return cn, nil
}

type Driver struct {
	configMutex sync.Mutex
	configs     map[string]*Conn
}

func (d *Driver) Open(name string) (driver.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	connector, _ := d.OpenConnector(name)
	return connector.Connect(ctx)
}

func (d *Driver) OpenConnector(name string) (driver.Connector, error) {
	cliOpts, err := ParseConfig(name)
	if err != nil {
		return nil, err
	}
	return &Connector{driver: d, cliOptions: cliOpts}, nil
}

func (dc *Connector) Driver() driver.Driver {
	return dc.driver
}

type Conn struct {
	conn    client.ImmuClient
	options *client.Options
	Token   string
}

// Conn returns the underlying client.ImmuClient
func (c *Conn) GetImmuClient() client.ImmuClient {
	return c.conn
}

func (c *Conn) GetToken() string {
	return c.Token
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	return nil, ErrNotImplemented
}

func (c *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return nil, ErrNotImplemented
}

func (c *Conn) Close() error {
	return nil
}

func (c *Conn) Begin() (driver.Tx, error) {
	return nil, ErrNotImplemented
}

func (c *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return nil, ErrNotImplemented
}

func (c *Conn) ExecContext(ctx context.Context, query string, argsV []driver.NamedValue) (driver.Result, error) {
	if !c.conn.IsConnected() {
		return nil, driver.ErrBadConn
	}

	md := metadata.Pairs("authorization", c.Token)
	ctx = metadata.NewOutgoingContext(ctx, md)

	vals, err := namedValuesToSqlMap(argsV)
	if err != nil {
		return nil, err
	}

	execResult, err := c.conn.SQLExec(ctx, query, vals)
	if err != nil {
		return nil, err
	}
	return driver.RowsAffected(execResult.UpdatedRows), err
}

func (c *Conn) QueryContext(ctx context.Context, query string, argsV []driver.NamedValue) (driver.Rows, error) {
	if !c.conn.IsConnected() {
		return nil, driver.ErrBadConn
	}
	md := metadata.Pairs("authorization", c.Token)
	ctx = metadata.NewOutgoingContext(ctx, md)

	vals, err := namedValuesToSqlMap(argsV)
	if err != nil {
		return nil, err
	}

	queryResult, err := c.conn.SQLQuery(ctx, query, vals, true)
	if err != nil {
		return nil, err
	}

	return &Rows{conn: c, rows: queryResult.Rows}, nil
}

func (c *Conn) Ping(ctx context.Context) error {
	if !c.conn.IsConnected() {
		return driver.ErrBadConn
	}
	err := c.conn.HealthCheck(ctx)
	if err != nil {
		c.Close()
		return driver.ErrBadConn
	}
	return nil
}

func (c *Conn) CheckNamedValue(nv *driver.NamedValue) error {
	// driver.Valuer interface is used instead
	return nil
}

func (c *Conn) ResetSession(ctx context.Context) error {
	if !c.conn.IsConnected() {
		return driver.ErrBadConn
	}
	return ErrNotImplemented
}

type Rows struct {
	index uint64
	conn  *Conn
	rows  []*schema.Row
}

func (r *Rows) Columns() []string {
	if len(r.rows) > 0 {
		names := make([]string, 0)
		for _, n := range r.rows[0].Columns {
			name := n[strings.LastIndex(n, ".")+1 : len(n)-1]
			names = append(names, string(name))
		}
		return names
	}
	return nil
}

// ColumnTypeDatabaseTypeName
func (r *Rows) ColumnTypeDatabaseTypeName(index int) string {
	return ""
}

// ColumnTypeLength If length is not limited other than system limits, it should return math.MaxInt64
func (r *Rows) ColumnTypeLength(index int) (int64, bool) {
	return math.MaxInt64, false
}

// ColumnTypePrecisionScale should return the precision and scale for decimal
// types. If not applicable, ok should be false.
func (r *Rows) ColumnTypePrecisionScale(index int) (precision, scale int64, ok bool) {
	return 0, 0, false
}

// ColumnTypeScanType returns the value type that can be used to scan types into.
func (r *Rows) ColumnTypeScanType(index int) reflect.Type {
	return nil
}

func (r *Rows) Close() error {
	return r.conn.Close()
}

func (r *Rows) Next(dest []driver.Value) error {
	if r.index >= uint64(len(r.rows)) {
		return io.EOF
	}

	row := r.rows[r.index]

	for idx, val := range row.Values {
		dest[idx] = schema.RenderValueAsByte(val.Value)
	}

	r.index++
	return nil
}

func namedValuesToSqlMap(argsV []driver.NamedValue) (map[string]interface{}, error) {
	args := make([]interface{}, 0, len(argsV))
	for _, v := range argsV {
		if v.Value != nil {
			args = append(args, v.Value.(interface{}))
		} else {
			args = append(args, nil)
		}
	}

	args, err := convertDriverValuers(args)
	if err != nil {
		return nil, err
	}

	vals := make(map[string]interface{})

	for id, nv := range args {
		key := "param" + strconv.Itoa(id+1)
		switch args[id].(type) {
		case string:
			vals[key] = nv
		case time.Time:
			vals[key] = nv.(time.Time).Unix()
		case uint:
			vals[key] = int64(nv.(uint))
		case float64:
			return nil, ErrFloatValuesNotSupported
		case interface{}:
			vals[key] = nv
		default:
			vals[key] = nv
		}
	}
	return vals, nil
}

func convertDriverValuers(args []interface{}) ([]interface{}, error) {
	for i, arg := range args {
		switch arg := arg.(type) {
		case driver.Valuer:
			v, err := callValuerValue(arg)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
	}
	return args, nil
}

var valuerReflectType = reflect.TypeOf((*driver.Valuer)(nil)).Elem()

// callValuerValue returns vr.Value()
// This function is mirrored in the database/sql/driver package.
func callValuerValue(vr driver.Valuer) (v driver.Value, err error) {
	if rv := reflect.ValueOf(vr); rv.Kind() == reflect.Ptr &&
		rv.IsNil() &&
		rv.Type().Elem().Implements(valuerReflectType) {
		return nil, nil
	}
	return vr.Value()
}

func ParseConfig(uri string) (*client.Options, error) {
	if uri != "" && strings.HasPrefix(uri, "immudb://") {
		url, err := url.Parse(uri)
		if err != nil {
			return nil, ErrBadQueryString
		}
		pw, _ := url.User.Password()
		port, err := strconv.Atoi(url.Port())
		if err != nil {
			return nil, ErrBadQueryString
		}
		cliOpts := client.DefaultOptions().
			WithUsername(url.User.Username()).
			WithPassword(pw).
			WithPort(port).
			WithAddress(url.Hostname()).
			WithDatabase(url.Path[1:])

		return cliOpts, nil
	}
	return nil, ErrBadQueryString
}

func GetUri(o *client.Options) (string, error) {
	uri := strings.Join([]string{"immudb://", o.Username, ":", o.Password, "@", o.Address, ":", strconv.Itoa(o.Port), "/", o.Database}, "")
	if _, err := ParseConfig(uri); err != nil {
		return "", errors.New("invalid client options")
	}
	return uri, nil
}