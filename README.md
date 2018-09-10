![DALí Logo](dali.png) &nbsp;
[![GoDoc](https://godoc.org/github.com/mibk/dali?status.png)](https://godoc.org/github.com/mibk/dali)
[![Build Status](https://travis-ci.org/mibk/dali.png)](https://travis-ci.org/mibk/dali)

# Database Abstraction Layer (í)

DALí is not exactly a database abstration layer. It doesn't try to abstract the SQL in a way
that the queries could run unchanged on any supported database. It rather abstracts
just the placeholder manipulation and provides convenient ways for some common situations.

The main goal of this project is to provide a clean, compact API for communication with
SQL databases.

## Quickstart

```go
package main

import (
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mibk/dali"
)

func main() {
	db, err := dali.Open("mysql", "root@/example?parseTime=true")
	if err != nil {
		log.Fatal(err)
	}

	q := db.Query(`INSERT INTO [group] ?values`, dali.Map{"name": "admins"})
	groupID, err := dali.LastInsertID(q.Exec())
	if err != nil {
		log.Fatal(err)
	}
	// INSERT INTO `group` (`name`) VALUES ('admins')

	users := []User{
		{0, "Peter", "peter@foo.com", groupID, time.Now()},
		{0, "Nick", "nick@bar.org", groupID, time.Now()},
	}
	_, err = db.Query(`INSERT INTO [user] ?values...`, users).Exec()
	if err != nil {
		log.Fatal(err)
	}
	// ?values... expands a slice of struct into multi insert
	// INSERT INTO `user` (`name`, `email`, `group_id`, `created`) VALUES
	//	('Peter', 'peter@foo.com', 1, '2015-11-20 13:59:59'),
	//	('Nick', 'nick@bar.org', 1, '2015-11-20 13:59:59')

	var u User
	q = db.Query(`SELECT * FROM ?ident WHERE group_id IN (?...) LIMIT 1`,
		"user", []int{1, 2, 5})
	fmt.Println(q) // dali.Query implements fmt.Stringer. It prints:
	// SELECT * FROM `user` WHERE group_id IN (1, 2, 5) LIMIT 1
	if err := q.One(&u); err != nil {
		log.Fatal(err)
	}
	fmt.Println(u)

	u.Email = "peter@foo.net"
	u.GroupID = 2
	_, err = db.Query(`UPDATE [user] ?set WHERE [id] = ?`,
		dali.Map{
			"email":    u.Email,
			"group_id": u.GroupID,
		}, 1).Exec()
	if err != nil {
		log.Fatal(err)
	}
	// UPDATE `user` SET `email` = 'peter@foo.net', `group_id` = 2
	//	WHERE `id` = 1
}

type User struct {
	ID         int64 `db:",selectonly"` // omitted on INSERT or UPDATE
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
So there is one way to escape identifiers among all dialects.

### Handy placeholders

Again, placeholder manipulation is the same for all dialects and besides that it also provides
some additional placeholders. The complete list is:

```
?          primitive value or a value implementing driver.Valuer
?...       a slice of values which is going to be expanded (especially useful in
           IN clauses)
?values    expects either Map, or a struct as an argument. It derives column names
           from map keys or struct fields and constructs a VALUES clause (e.g.
           INSERT INTO user ?values)
?set       similar to ?values but used for SET clauses (e.g. UPDATE user SET ?set)
?values... expects a slice of structs as an argument which is expanded into multi
           INSERT clause
?ident     used for identifiers (column or table name)
?ident...  expands identifiers and separates them with a comma
?sql       inserts the parameter as is (meant for SQL parts)
```

Using the placeholders it is easy and quite expressive to write common SQL queries, but it is
also possible to adjust these queries to a specific need (which is often not so easy when using
query builders).

*Note*: only `?`, `?ident`, `?ident...`, and `?sql` are allowed in prepared statements (see Prepare method's
doc for more information).

### Profiling and other

Using the [DB.SetMiddlewareFunc](https://godoc.org/github.com/mibk/dali#DB.SetMiddlewareFunc) it is
possible to do additional operations before and after execution of every query. This example
logs every executed query:

```go
var db *dali.DB // init db...

func init() {
	db.SetMiddlewareFunc(profile)
}

func profile(e dali.Execer) dali.Execer {
	return profiler{e}
}

type profiler struct {
	ex dali.Execer
}

func (p profiler) Exec(query string, args ...interface{}) (sql.Result, error) {
	log.Println(query, args)
	return p.ex.Exec(query, args...)
}

func (p profiler) Query(query string, args ...interface{}) (*sql.Rows, error) {
	log.Println(query, args)
	return p.ex.Query(query, args...)
}

func (p profiler) QueryRow(query string, args ...interface{}) *sql.Row {
	log.Println(query, args)
	return p.ex.QueryRow(query, args...)
}
```

### Faster performance

DALí interpolates all parameters before it gets to the database which has a huge performance
benefit. This behaviour is taken from the **gocraft/dbr** library. See
[this](https://github.com/gocraft/dbr#faster-performance-than-using-using-databasesql-directly)
for more information.

### Supported dialects

Currently, only a MySQL dialect is implemented directly in this package (see [dialects](dialects)
for more information). Nevertheless, supporting another dialect should be as easy as creating
a new dialect implementing *dialects.Dialect* interface. The most common dialects will be
implemented directly in the future.

## Thanks

Ideas for building this library come mainly from these sources:

- [gocraft/dbr](https://github.com/gocraft/dbr) for interpolation, loading methods and other
- [nextras/dbal](https://github.com/nextras/dbal) for the placeholders (although it is a PHP library)
- [jmoiron/sqlx](https://github.com/jmoiron/sqlx) for general ideas

## License

DALí is distributed under the MIT license found in the [LICENSE](LICENSE) file.
