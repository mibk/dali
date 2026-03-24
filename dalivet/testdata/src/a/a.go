package a

import (
	"context"
	"database/sql/driver"
	"maps"
	"slices"
	"time"

	"github.com/mibk/dali"
)

type User struct {
	ID   int
	Name string
}

type MyTime time.Time

type Valuer struct{}

func (Valuer) Value() (driver.Value, error) { return nil, nil }

type SQLMarshaler struct{}

func (SQLMarshaler) MarshalSQL(t dali.Translator) (string, error) { return "1", nil }

func checkOne(q *dali.Query) {
	var u User
	q.One(&u)       // OK
	q.One(u)        // want `One requires a pointer to a struct, got a\.User`
	q.One(new(int)) // want `One requires a pointer to a struct, got \*int`
	var s string
	q.One(&s) // want `One requires a pointer to a struct, got \*string`
}

func checkAll(q *dali.Query) {
	var users []User
	q.All(&users) // OK
	var ptrs []*User
	q.All(&ptrs) // OK
	q.All(users) // want `All requires a pointer to a slice of structs, got \[\]a\.User`
	var ints []int
	q.All(&ints) // want `All requires a pointer to a slice of structs, got \*\[\]int`
	var pptrs []**User
	q.All(&pptrs) // want `All does not allow pointer to pointer as slice element, got \*\[\]\*\*a\.User`
}

func checkScanAllRows(q *dali.Query) {
	var ids []int
	var names []string
	q.ScanAllRows(&ids, &names) // OK
	q.ScanAllRows(ids)          // want `ScanAllRows requires a pointer to a slice, got \[\]int`
	var n int
	q.ScanAllRows(&n) // want `ScanAllRows requires a pointer to a slice, got \*int`
}

func checkArgCount(db *dali.DB) {
	db.Query("SELECT ?", 1)       // OK
	db.Query("SELECT ?, ?", 1, 2) // OK
	db.Query("SELECT ?, ?", 1)    // want `DB\.Query has 2 placeholder\(s\) but 1 arg\(s\)`
	db.Query("SELECT ?", 1, 2)    // want `DB\.Query has 1 placeholder\(s\) but 2 arg\(s\)`
	db.Query("SELECT 1")          // OK: no placeholders, no args
}

func checkArgCountPrepare(db *dali.DB) {
	db.Prepare("SELECT ? WHERE [col] = ?")                  // OK: ? in Prepare doesn't consume args
	db.Prepare("SELECT ? WHERE ?ident = ?", "col")          // OK: ?ident consumes 1 arg
	db.Prepare("SELECT ? WHERE ?ident = ?", "col", "extra") // want `DB\.Prepare has 1 placeholder\(s\) but 2 arg\(s\)`
}

func checkPlaceholderTypes(db *dali.DB) {
	db.Query("SELECT ?", 1)          // OK: int is scalar
	db.Query("SELECT ?", "hello")    // OK: string is scalar
	db.Query("SELECT ?", true)       // OK: bool is scalar
	db.Query("SELECT ?", []byte{1})  // OK: []byte is fine
	db.Query("SELECT ?", time.Now()) // OK: time.Time is fine

	var u User
	db.Query("SELECT ?", u)        // want `\? does not accept structs`
	db.Query("SELECT ?", []int{1}) // want `\? does not accept slices`

	db.Query("SELECT ?...", []int{1, 2}) // OK: ?... wants a slice
	db.Query("SELECT ?...", 42)          // want `\?\.\.\. requires a slice, got int`

	db.Query("SELECT ?ident", "col") // OK
	db.Query("SELECT ?ident", 42)    // want `\?ident requires a string, got int`

	db.Query("SELECT ?ident...", []string{"a"}) // OK
	db.Query("SELECT ?ident...", []int{1})      // want `\?ident\.\.\. requires \[\]string, got \[\]int`

	db.Query("INSERT ?values", &u)               // OK: struct
	db.Query("INSERT ?values", dali.Map{"a": 1}) // OK: dali.Map
	db.Query("INSERT ?values", 42)               // want `\?values requires a struct or dali\.Map, got int`

	db.Query("UPDATE ?set", &u) // OK
	db.Query("UPDATE ?set", 42) // want `\?set requires a struct or dali\.Map, got int`

	db.Query("SELECT ?sql", "raw") // OK
	var m SQLMarshaler
	db.Query("SELECT ?sql", m)  // OK
	db.Query("SELECT ?sql", 42) // want `\?sql requires a string or dali\.Marshaler, got int`
}

func checkInvalidPlaceholders(db *dali.DB) {
	db.Query("SELECT ?foo", 1) // want `unknown placeholder \?foo`
}

func checkPrepareRestrictions(db *dali.DB) {
	db.Prepare("SELECT ?... WHERE x = ?", []int{1})       // want `\?\.\.\. cannot be used in prepared statements`
	db.Prepare("INSERT ?values WHERE x = ?", User{})      // want `\?values cannot be used in prepared statements`
	db.Prepare("INSERT ?values... WHERE x = ?", []User{}) // want `\?values\.\.\. cannot be used in prepared statements`
	db.Prepare("UPDATE ?set WHERE x = ?", User{})         // want `\?set cannot be used in prepared statements`
}

func checkEmptySliceLiteral(db *dali.DB) {
	db.Query("SELECT ?...", []int{})         // want `empty slice passed to \?\.\.\.`
	db.Query("SELECT ?ident...", []string{}) // want `empty slice passed to \?ident\.\.\.`
	db.Query("SELECT ?values...", []User{})  // want `empty slice passed to \?values\.\.\.`
	db.Query("SELECT ?...", []int{1})        // OK
}

func checkUnguardedSlice(db *dali.DB) {
	items := []int{1, 2}
	db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`

	idents := []string{"a"}
	db.Query("SELECT ?ident...", idents) // want `slice "idents" passed to \?ident\.\.\. without a length check`
}

func checkGuardedBailout(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		return
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedBailoutPanic(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		panic("unreachable")
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedPositive(db *dali.DB) {
	items := []int{1, 2}
	if len(items) > 0 {
		db.Query("SELECT ?...", items) // OK
	}
}

func checkGuardedNotEqual(db *dali.DB) {
	items := []int{1, 2}
	if len(items) != 0 {
		db.Query("SELECT ?...", items) // OK
	}
}

func checkGuardedDefault(db *dali.DB) {
	items := []int64{1, 2}
	if len(items) == 0 {
		items = []int64{-1}
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedCompound(db *dali.DB) {
	items := []int{1, 2}
	var id *int
	if id != nil && len(items) > 0 {
		db.Query("SELECT ?...", items) // OK
	}
	_ = id
}

func checkGuardedLessThanOne(db *dali.DB) {
	items := []int{1, 2}
	if len(items) < 1 {
		return
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedElseIfBailout(db *dali.DB) error {
	items, err := makeItems()
	if err != nil {
		return err
	} else if len(items) == 0 {
		return nil
	}
	db.Query("SELECT ?...", items) // OK
	return nil
}

func checkGuardedElseIfBailoutPanic(db *dali.DB) {
	items, err := makeItems()
	if err != nil {
		panic(err)
	} else if len(items) == 0 {
		panic("empty")
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedElseIfDefault(db *dali.DB) error {
	items, err := makeItems()
	if err != nil {
		return err
	} else if len(items) == 0 {
		items = []int{-1}
	}
	db.Query("SELECT ?...", items) // OK
	return nil
}

func checkUnguardedElseIfNonTerminating(db *dali.DB) {
	items, _ := makeItems()
	if items == nil {
		items = []int{} // does not terminate
	} else if len(items) == 0 {
		return
	}
	db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
}

func makeItems() ([]int, error) { return nil, nil }

func checkGuardedInClosure(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		return
	}
	f := func() {
		db.Query("SELECT ?...", items) // OK
	}
	f()
}

func checkGuardedTransitiveRange(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		return
	}
	var ids []int64
	for _, i := range items {
		ids = append(ids, int64(i))
	}
	db.Query("SELECT ?...", ids) // OK
}

func checkGuardedTransitiveRangeCompact(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		return
	}
	var ids []int64
	for _, i := range items {
		ids = append(ids, int64(i))
	}
	ids = slices.Compact(ids)
	db.Query("SELECT ?...", ids) // OK
}

func checkGuardedTransitiveRangeNestedBlock(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		return
	}
	var ids []int64
	for _, i := range items {
		ids = append(ids, int64(i))
	}
	{
		ids = slices.Compact(ids)
		db.Query("SELECT ?...", ids) // OK
	}
}

func checkUnguardedTransitiveRange(db *dali.DB) {
	items := []int{1, 2}
	var ids []int64
	for _, i := range items {
		ids = append(ids, int64(i))
	}
	db.Query("SELECT ?...", ids) // want `slice "ids" passed to \?\.\.\. without a length check`
}

func checkGuardedMakeLen(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		return
	}
	ids := make([]int64, len(items))
	for i, v := range items {
		ids[i] = int64(v)
	}
	db.Query("SELECT ?...", ids) // OK
}

func checkUnguardedMakeLen(db *dali.DB) {
	items := []int{1, 2}
	ids := make([]int64, len(items))
	for i, v := range items {
		ids[i] = int64(v)
	}
	db.Query("SELECT ?...", ids) // want `slice "ids" passed to \?\.\.\. without a length check`
}

func checkValuer(db *dali.DB) {
	var v Valuer
	db.Query("SELECT ?", v) // OK: implements driver.Valuer
}

func checkInterface(q *dali.Query) {
	var x any
	q.One(x) // OK: interface, skip check
}

func checkTx(tx *dali.Tx) {
	tx.Query("SELECT ?, ?", 1) // want `Tx\.Query has 2 placeholder\(s\) but 1 arg\(s\)`
}

func checkNonConstantQuery(db *dali.DB, tx *dali.Tx) {
	sql := "SELECT 1"
	db.Query(sql)   // want `DB\.Query requires a constant query string`
	tx.Query(sql)   // want `Tx\.Query requires a constant query string`
	db.Prepare(sql) // want `DB\.Prepare requires a constant query string`
	tx.Prepare(sql) // want `Tx\.Prepare requires a constant query string`

	ctx := context.Background()
	db.QueryWithContext(ctx, sql) // want `DB\.QueryWithContext requires a constant query string`
	tx.QueryWithContext(ctx, sql) // want `Tx\.QueryWithContext requires a constant query string`
	db.PrepareContext(ctx, sql)   // want `DB\.PrepareContext requires a constant query string`
	tx.PrepareContext(ctx, sql)   // want `Tx\.PrepareContext requires a constant query string`

	const q = "SELECT 1"
	db.Query(q)   // OK: const
	tx.Query(q)   // OK: const
	db.Prepare(q) // OK: const
	tx.Prepare(q) // OK: const
}

func checkGuardedDerived(db *dali.DB) {
	m := map[int]bool{1: true}
	if len(m) > 0 {
		ids := slices.Sorted(maps.Keys(m))
		db.Query("SELECT ?...", ids) // OK
	}
}

func checkUnguardedDerived(db *dali.DB) {
	m := map[int]bool{1: true}
	ids := slices.Sorted(maps.Keys(m))
	db.Query("SELECT ?...", ids) // want `slice "ids" passed to \?\.\.\. without a length check`
}

func checkGuardedDerivedViaMapPopulation(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		return
	}
	dedup := make(map[int]bool)
	for _, v := range items {
		dedup[v] = true
	}
	ids := slices.Sorted(maps.Keys(dedup))
	db.Query("SELECT ?...", ids) // OK
}

func checkGuardedDerivedViaMapPopulationClosure(db *dali.DB) {
	items := []int{1, 2}
	if len(items) == 0 {
		return
	}
	f := func() {
		dedup := make(map[int]bool)
		for _, v := range items {
			dedup[v] = true
		}
		ids := slices.Sorted(maps.Keys(dedup))
		db.Query("SELECT ?...", ids) // OK
	}
	f()
}

func checkGuardedDerivedViaSelectorRangeMapChain(db *dali.DB) {
	var v struct{ Items []int }
	if len(v.Items) == 0 {
		return
	}
	dedup := make(map[int]bool)
	for _, x := range v.Items {
		dedup[x] = true
	}
	ids := slices.Sorted(maps.Keys(dedup))
	db.Query("SELECT ?...", ids) // OK
}
