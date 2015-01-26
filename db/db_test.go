package db

import (
	"reflect"
	"testing"
)

type Dog struct {
	Name string `sql:"name"`
}

type Person struct {
	TableName string `sql:"people"`
	Name      string `sql:"name"`
	Age       int    `sql:"age"`
	Weight    float64
	state     bool
}

func TestGeneratePreparedInsert(t *testing.T) {
	expectedSQL := "INSERT INTO people (name, age, weight) VALUES ($1, $2, $3)"
	expectedValues := []interface{}{"Gopher", 30, 30.04}
	p := Person{Name: "Gopher", Age: 30, Weight: 30.04}
	sql, values := generatePreparedInsert(p)
	if sql != expectedSQL {
		t.Errorf("generatePreparedInsert: want %q. Got %q", expectedSQL, sql)
	}
	if !reflect.DeepEqual(values, expectedValues) {
		t.Errorf("generatePreparedInsert: want %v. Got %v", expectedValues, values)
	}
}

func TestGeneratePreparedInsertPointer(t *testing.T) {
	expectedSQL := "INSERT INTO people (name, age, weight) VALUES ($1, $2, $3)"
	expectedValues := []interface{}{"Gopher", 30, 30.04}
	p := Person{Name: "Gopher", Age: 30, Weight: 30.04}
	sql, values := generatePreparedInsert(&p)
	if sql != expectedSQL {
		t.Errorf("generatePreparedInsert: want %q. Got %q", expectedSQL, sql)
	}
	if !reflect.DeepEqual(values, expectedValues) {
		t.Errorf("generatePreparedInsert: want %v. Got %v", expectedValues, values)
	}
}

func TestGeneratePreparedInsertInvalid(t *testing.T) {
	sql, values := generatePreparedInsert(10)
	if sql != "" {
		t.Errorf("generatePreparedInsert: want %q. Got %q", "", sql)
	}
	if values != nil {
		t.Errorf("generatePreparedInsert: want nil. Got %v", values)
	}
}

func TestGeneratePreparedInsertNil(t *testing.T) {
	sql, values := generatePreparedInsert(nil)
	if sql != "" {
		t.Errorf("generatePreparedInsert: want %q. Got %q", "", sql)
	}
	if values != nil {
		t.Errorf("generatePreparedInsert: want nil. Got %v", values)
	}
}

func TestGeneratePreparedInsertUntaggedTable(t *testing.T) {
	expectedSQL := "INSERT INTO dog (name) VALUES ($1)"
	expectedValues := []interface{}{"toto"}
	d := Dog{Name: "toto"}
	sql, values := generatePreparedInsert(d)
	if sql != expectedSQL {
		t.Errorf("generatePreparedInsert: want %q. Got %q", expectedSQL, sql)
	}
	if !reflect.DeepEqual(values, expectedValues) {
		t.Errorf("generatePreparedInsert: want %v. Got %v", expectedValues, values)
	}
}
