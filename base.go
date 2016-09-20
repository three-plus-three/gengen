package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/Masterminds/squirrel"
	"github.com/lann/builder"
)

var ErrNotUpdated = errors.New("no record is updated")
var ErrNotDeleted = errors.New("no record is deleted")

func isPostgersql(db interface{}) bool {
	return true
}

func isPlaceholderWithDollar(value interface{}) bool {
	return true
}

type ViewModel struct {
	TableName   string
	ColumnNames []string
}

func (viewModel *ViewModel) Count(db squirrel.QueryRower) (count int64, err error) {
	selectBuilder := viewModel.Where().Select("count(*)").From(viewModel.TableName)
	err = squirrel.QueryRowWith(db, selectBuilder).Scan(&count)
	return
}

func (viewModel *ViewModel) Where(exprs ...Expr) squirrel.StatementBuilderType {
	if len(exprs) == 0 {
		return squirrel.StatementBuilder
	}
	if len(exprs) == 1 {
		return builder.Append(squirrel.StatementBuilder, "WhereParts", exprs[0]).(squirrel.StatementBuilderType)
	}
	sqlizers := make([]squirrel.Sqlizer, 0, len(exprs))
	for _, exp := range exprs {
		sqlizers = append(sqlizers, exp)
	}

	return builder.Append(squirrel.StatementBuilder, "WhereParts", squirrel.And(sqlizers)).(squirrel.StatementBuilderType)
}

func (viewModel *ViewModel) UpdateBy(db squirrel.BaseRunner, values map[string]interface{}, pred interface{}, args ...interface{}) (int64, error) {
	sql := squirrel.Update(viewModel.TableName)
	if isPlaceholderWithDollar(db) {
		sql = sql.PlaceholderFormat(squirrel.Dollar)
	}

	for key, value := range values {
		sql = sql.Set(key, value)
	}

	sql = sql.Where(pred, args)

	result, e := sql.RunWith(db).Exec()
	if nil != e {
		return 0, e
	}
	return result.RowsAffected()
}

func (viewModel *ViewModel) Delete(db squirrel.BaseRunner, exprs ...Expr) (int64, error) {
	sq := viewModel.Where(exprs...).Delete(viewModel.TableName)
	if isPlaceholderWithDollar(db) {
		sq = sq.PlaceholderFormat(squirrel.Dollar)
	}
	result, e := sq.RunWith(db).Exec()
	if nil != e {
		return 0, e
	}
	return result.RowsAffected()
}

func (viewModel *ViewModel) DeleteBy(db squirrel.BaseRunner, pred interface{}, args ...interface{}) (int64, error) {
	sq := squirrel.Delete(viewModel.TableName).Where(pred, args)
	if isPlaceholderWithDollar(db) {
		sq = sq.PlaceholderFormat(squirrel.Dollar)
	}

	result, e := sq.RunWith(db).Exec()
	if nil != e {
		return 0, e
	}
	return result.RowsAffected()
}

type DbModel struct {
	ViewModel
	KeyNames []string
}

func (dbModel *DbModel) UpdateByPrimaryKey(db squirrel.BaseRunner, values map[string]interface{}, keys ...interface{}) error {
	sql := squirrel.Update(dbModel.TableName)
	if isPlaceholderWithDollar(db) {
		sql = sql.PlaceholderFormat(squirrel.Dollar)
	}

	for key, value := range values {
		sql = sql.Set(key, value)
	}

	cond := squirrel.Eq{}
	for idx, key := range keys {
		cond[dbModel.KeyNames[idx]] = key
	}
	sql = sql.Where(cond)

	result, e := sql.RunWith(db).Exec()
	if nil != e {
		return e
	}

	rowsAffected, e := result.RowsAffected()
	if nil != e {
		return e
	}

	if 0 == rowsAffected {
		return ErrNotUpdated
	}
	return nil
}

func (dbModel *DbModel) DeleteByPrimaryKey(db squirrel.BaseRunner, keys ...interface{}) error {
	sql := squirrel.Delete(dbModel.TableName)
	if isPlaceholderWithDollar(db) {
		sql = sql.PlaceholderFormat(squirrel.Dollar)
	}
	cond := squirrel.Eq{}
	for idx, key := range keys {
		cond[dbModel.KeyNames[idx]] = key
	}

	result, e := sql.Where(cond).RunWith(db).Exec()
	if nil != e {
		return e
	}
	rowsAffected, e := result.RowsAffected()
	if nil != e {
		return e
	}

	if 0 == rowsAffected {
		return ErrNotDeleted
	}
	return nil
}

type ColumnModel struct {
	Name string
}

func (model *ColumnModel) EQU(value interface{}) Expr {
	return Expr{Column: model, Operator: "=", Value: value}
}

func (model *ColumnModel) IN(values ...interface{}) Expr {
	if len(values) == 0 {
		panic(errors.New("values is empty."))
	}
	return Expr{Column: model, Operator: "IN", Value: values}
}

func (model *ColumnModel) NEQ(value interface{}) Expr {
	return Expr{Column: model, Operator: "<>", Value: value}
}

func (model *ColumnModel) EXISTS(value interface{}) Expr {
	return Expr{Column: model, Operator: "EXISTS", Value: value}
}

func (self *ColumnModel) LIKE(value string) Expr {
	return Expr{Column: self, Operator: "LIKE", Value: value}
}

type Expr struct {
	Column   *ColumnModel
	Operator string
	Value    interface{}
}

func (model Expr) ToSql() (string, []interface{}, error) {
	if sqlizer, ok := model.Value.(squirrel.Sqlizer); ok {
		sub_sqlstr, sub_args, e := sqlizer.ToSql()
		if nil != e {
			return "", nil, e
		}
		return model.Column.Name + " " + model.Operator + " " + sub_sqlstr, sub_args, nil
	}
	if "IN" == model.Operator {
		var buf bytes.Buffer
		buf.WriteString(model.Column.Name)
		buf.WriteString(" IN (")
		JoinObjects(&buf, model.Value)
		buf.Truncate(buf.Len() - 1)
		buf.WriteString(") ")
		return buf.String(), nil, nil
	}
	return model.Column.Name + " " + model.Operator + " ? ", []interface{}{model.Value}, nil
}

func JoinObjects(buf *bytes.Buffer, value interface{}) {
	if inner, ok := value.([]interface{}); ok {
		for _, v := range inner {
			JoinObjects(buf, v)
		}
	} else if inner, ok := value.([]uint); ok {
		for _, v := range inner {
			buf.WriteString(fmt.Sprint(v))
			buf.WriteString(",")
		}
	} else if inner, ok := value.([]int); ok {
		for _, v := range inner {
			buf.WriteString(fmt.Sprint(v))
			buf.WriteString(",")
		}
	} else if inner, ok := value.([]uint64); ok {
		for _, v := range inner {
			buf.WriteString(fmt.Sprint(v))
			buf.WriteString(",")
		}
	} else if inner, ok := value.([]int64); ok {
		for _, v := range inner {
			buf.WriteString(fmt.Sprint(v))
			buf.WriteString(",")
		}
	} else {
		valVal := reflect.ValueOf(value)
		if valVal.Kind() == reflect.Array || valVal.Kind() == reflect.Slice {
			for i := 0; i < valVal.Len(); i++ {
				buf.WriteString(fmt.Sprint(valVal.Index(i).Interface()))
				buf.WriteString(",")
			}
		} else {
			buf.WriteString(fmt.Sprint(value))
			buf.WriteString(",")
		}
	}
}

// Sqlizer is the interface that wraps the ToSql method.
//
// ToSql returns a SQL representation of the Sqlizer, along with a slice of args
// as passed to e.g. database/sql.Exec. It can also return an error.
type Sqlizer interface {
	ToSql() (string, []interface{}, error)
}

// Execer is the interface that wraps the Exec method.
//
// Exec executes the given query as implemented by database/sql.Exec.
type Execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// Queryer is the interface that wraps the Query method.
//
// Query executes the given query as implemented by database/sql.Query.
type Queryer interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// QueryRower is the interface that wraps the QueryRow method.
//
// QueryRow executes the given query as implemented by database/sql.QueryRow.
type QueryRower interface {
	QueryRow(query string, args ...interface{}) RowScanner
}

// BaseRunner groups the Execer and Queryer interfaces.
type BaseRunner interface {
	Execer
	Queryer
}

// Runner groups the Execer, Queryer, and QueryRower interfaces.
type Runner interface {
	Execer
	Queryer
	QueryRower
}

// RowScanner is the interface that wraps the Scan method.
//
// Scan behaves like database/sql.Row.Scan.
type RowScanner interface {
	Scan(...interface{}) error
}

// Row wraps database/sql.Row to let squirrel return new errors on Scan.
type Row struct {
	RowScanner
	err error
}

// Scan returns Row.err or calls RowScanner.Scan.
func (r *Row) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	return r.RowScanner.Scan(dest...)
}
