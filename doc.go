// Package dali wraps the sql.DB and provides convenient API for building
// database driven applications. Its main goal is to create a unified way
// of handling placeholders among all drivers and to simplify some common,
// repetive queries.
//
// There is no support for query builders (you have to write pure SQL queries).
// It focuses on the common queries (like writing INSERTs or UPDATEs) and on
// loading of results into structs, for which it provides easy-to-write alternatives.
//
// Placeholders
//
// The following is the complete list of possible placeholders that can be used
// when writing a query using Query method.
//
//   ?          primitive value or a value implementing driver.Valuer
//   ?...       a slice of values which is going to be expanded (especially useful in
//              IN clauses)
//   ?values    expects either Map, or a struct as an argument. It derives column names
//              from map keys or struct fields and constructs a VALUES clause (e.g.
//              INSERT INTO user ?values)
//   ?set       similar to ?values but used for SET clauses (e.g. UPDATE user SET ?set)
//   ?values... expects a slice of structs as an argument which is expanded into multi
//              INSERT clause
//   ?ident     used for identifiers (column or table name)
//   ?ident...  expands identifiers and separates them with a comma
//   ?raw       inserts the parameter as is (meant for SQL parts)
//
// Prepared statements
//
// dali has also a support for prepared statements. However, it doesn't support certain
// placeholders. Only ?ident, ?ident..., and ?raw placeholders are allowed in the phase
// of the query building (befored the statement is prepared). The ? placeholder is the
// only one left for parameter binding. So working with prepared statements can look
// like this:
//
// 	cols := []strings{"name", "group_id"}
// 	var (
// 		name    string
// 		groupID int64
// 	)
// 	stmt := db.Prepare(`SELECT ?ident... FROM [user] WHERE [id] = ?`, cols)
//	// This prepares this statement:
//	//	SELECT `name`, `group_id` FROM `user` WHERE `id` = ?
// 	stmt.Bind(14).ScanRow(&name, &groupID)
//	// Bind the statement with 14 value and scan the row into these variables.
package dali
