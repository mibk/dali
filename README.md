![DALí Logo](_doc/img/dali.png)
# Database Abstraction Layer (í) [![GoDoc](https://godoc.org/github.com/mibk/dali?status.png)](https://godoc.org/github.com/mibk/dali)

DALí is not exactly a database abstration layer. It doesn't try to abstract the SQL in a way
that the queries could run without changing on any supported driver. It rather abstracts
just placeholder manipulation and provides convenient ways for some common situations.

The main goal of this project is to provide a clean, compact API for comunication with
an SQL database.

## Status

At the moment, project is settling down, hoping, the v1.0 could be released in a month or so.

## Quickstart

```go
package main

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mibk/dali"
)

var conn = dali.MustOpenAndVerify("mysql", "root@/example?parseTime=true")

func main() {
	res := conn.Query(`INSERT INTO [group] ?values`, dali.Map{"name": "admins"}).
		MustExec()
	// INSERT INTO `group` (`name`) VALUES ('admins')

	groupID, _ := res.LastInsertId()
	users := []User{
		{0, "Peter", "peter@foo.com", groupID, time.Now()},
		{0, "Nick", "nick@bar.org", groupID, time.Now()},
	}
	conn.Query(`INSERT INTO [user] ?values...`, users).MustExec()
	// ?values... expands a slice of struct into multi insert

	var u User
	q := conn.Query(`SELECT * FROM user WHERE group_id IN (?...) LIMIT 1`,
		[]int64{1, 2, 5})
	fmt.Println(q) // the query implements fmt.Stringer
	if err := q.One(&u); err != nil {
		panic(err)
	}
	fmt.Println(u)
}

type User struct {
	ID         int64 `db:",omitinsert"` // omitted on INSERT or UPDATE
	Name       string
	Email      string
	GroupID    int64
	Registered time.Time `db:"created"`
}
```

## Instalation

```bash
$ go get github.com/mibk/dali
```

## Issues

DALí processes the query unaware of the actual SQL syntax. This means it is quite stupid
on deciding whether the placeholder is inside a string literal.
```go
conn.Query(`SELECT * FROM foo WHERE name = 'really?'`)
// This will return an error because it would try to replace the `?` with an argument
// that is missing.
```
To avoid this just use the whole string as a parameter.
```go
conn.Query(`SELECT * FROM foo WHERE name = ?`, "really?")
```

## Features

### Identifier escaping

This feature comes from the need to fix the clumsy way of escaping identifiers in MySQL in
Go's raw string literals. So instead of
```go
sql := `SELECT `+"`where`"+`
	FROM location`
```
you can use
```go
sql := `SELECT [where]
	FROM location
```
So there is one way to escape identifiers among all drivers.

### Handy placeholders

Again, placeholder manipulation is the same for all drivers and besides that it also provides
some additional placeholders. The complete list is:

```
?          primitive value or a value implementing driver.Valuer
?...       a slice of values which is going to be expanded (especially useful in
           IN clauses)
?values    expects as an argument either Map, or a struct. It derives column names
           from map keys or struct fields and constructs a VALUES clause (e.g. INSERT
           INTO user ?values)
?set       similar to ?values but used for SET clauses (e.g. UPDATE user SET ?set)
?values... expects as an argument a slice of structs which is expanded into multi
           INSERT clause
?ident     used for identifiers (column or table name)
?ident...  expands identifiers and separates them with a comma
?raw       inserts the parameter as is (meant for SQL parts)
```

### Faster performance

DALí interpolates all parameters before it gets to the database which has a huge performance
benefit. This behaviour is taken from the **gocraft/dbr** library. See
[this](https://github.com/gocraft/dbr#faster-performance-than-using-using-databasesql-directly)
for more information.

### Driver support

Currently, only a MySQL driver is implemented directly in this package (see [drivers](drivers)
for more information). Nevertheless supporting another driver should be as easy as creating
a new driver implementing *drivers.Driver* interface. In the future, there will be the most
common drivers implemented directly here.

## Thanks

Ideas for building this library come mainly from these sources:

- [gocraft/dbr](https://github.com/gocraft/dbr) for interpolation, loading methods and other
- [nextras/dbal](https://github.com/nextras/dbal) for the placeholders (although it is a PHP library)

## License

DALí is distributed under the MIT license found in the [LICENSE](LICENSE) file.
