// Package dali wraps the sql.DB and provides convenient API for building
// database driven applications. The main goal is to create a unified way
// of handling placeholders among all drivers, and to simplify some common,
// repetive queries.
//
// There is no support for query builders (you have to write pure SQL queries).
// It just focuses on the small amount of common queries (like writing INSERTs
// or UPDATEs) and on the loading of data into struct, for which it provides
// easy-to-write alternatives.
//
// Placeholders
//
// The following is the complete list of possible placeholders that can be used
// when writing a query using Query method.
//
//   ?          primitive value or a value implementing driver.Valuer
//   ?...       a slice of values which is going to be expanded (especially useful in
//              IN clauses)
//   ?values    expects as an argument either Map, or a struct. It derives column names
//              from map keys or struct fields and constructs a VALUES clause (e.g. INSERT
//              INTO user ?values)
//   ?set       similar to ?values but used for SET clauses (e.g. UPDATE user SET ?set)
//   ?values... expects as an argument a slice of structs which is expanded into multi
//              INSERT clause
//   ?ident     used for identifiers (column or table name)
//   ?ident...  expands identifiers and separates them with a comma
//   ?raw       inserts the parameter as is (meant for SQL parts)
package dali
