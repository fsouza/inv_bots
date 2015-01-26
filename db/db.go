package db

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

type Session struct {
	conn *sql.DB
}

func Connect(connectionString string) (*Session, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}
	session := Session{conn: db}
	return &session, nil
}

func (s *Session) Close() error {
	return s.conn.Close()
}

func (s *Session) Insert(record interface{}) error {
	sql, values := generatePreparedInsert(record)
	if sql == "" {
		return errors.New("invalid value")
	}
	_, err := s.conn.Exec(sql, values...)
	return err
}

func generatePreparedInsert(record interface{}) (string, []interface{}) {
	if record == nil {
		return "", nil
	}
	value := reflect.ValueOf(record)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return "", nil
	}
	var tableName string
	var fieldNames []string
	var placeHolders []string
	var fieldValues []interface{}
	var count int
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Name == "TableName" {
			tableName = field.Tag.Get("sql")
		} else {
			fieldName := field.Tag.Get("sql")
			if fieldName == "" {
				fieldName = strings.ToLower(field.Name)
			}
			count++
			fieldNames = append(fieldNames, fieldName)
			fieldValues = append(fieldValues, value.Field(i).Interface())
			placeHolders = append(placeHolders, "$" + strconv.Itoa(count))
		}
	}
	if tableName == "" {
		tableName = strings.ToLower(value.Type().Name())
	}
	fields := strings.Join(fieldNames, ", ")
	placeholders := strings.Join(placeHolders, ", ")
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, fields, placeholders)
	return sql, fieldValues
}
