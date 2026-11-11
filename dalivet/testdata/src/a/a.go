package a

import (
	"context"
	"database/sql/driver"
	"log"
	"maps"
	"os"
	"slices"
	"testing"
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

	db.Query("INSERT ?values", &dali.Map{"a": 1})       // want `\?values requires a struct or dali\.Map, got \*github\.com/mibk/dali\.Map`
	db.Query("INSERT ?values...", []dali.Map{{"a": 1}}) // want `\?values\.\.\. requires a slice of structs, got \[\]github\.com/mibk/dali\.Map`
	db.Query("INSERT ?values...", []*User{{}})          // OK

	db.Query("UPDATE ?set", &u) // OK
	db.Query("UPDATE ?set", 42) // want `\?set requires a struct or dali\.Map, got int`

	db.Query("SELECT ?sql", "raw") // OK
	var m SQLMarshaler
	db.Query("SELECT ?sql", m)  // OK
	db.Query("SELECT ?sql", 42) // want `\?sql requires a string or dali\.Marshaler, got int`
}

type (
	Names []string
	Col   string
)

func checkAssertedTypes(db *dali.DB) {
	const c = "col"
	db.Query("SELECT ?ident", c)             // OK
	db.Query("SELECT ?ident", Col("a"))      // want `\?ident requires a string, got a\.Col`
	db.Query("SELECT ?ident...", Names{"a"}) // want `\?ident\.\.\. requires \[\]string, got a\.Names`
	db.Query("SELECT ?ident...", []Col{"a"}) // want `\?ident\.\.\. requires \[\]string, got \[\]a\.Col`
	db.Query("SELECT ?sql", Col("a"))        // want `\?sql requires a string or dali\.Marshaler, got a\.Col`
	db.Query("SELECT ?sql", c)               // OK
}

func checkInvalidPlaceholders(db *dali.DB) {
	db.Query("SELECT ?foo", 1)             // want `unknown placeholder \?foo`
	db.Query("SELECT [broken")             // want `identifier not terminated`
	db.Query("UPDATE ?set...", User{})     // want `\?set cannot be expanded`
	db.Query("SELECT ?sql...", []string{}) // want `\?sql cannot be expanded`
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

func checkUnguardedSlice(db *dali.DB, items []int, idents []string) {
	db.Query("SELECT ?...", items)       // want `slice "items" passed to \?\.\.\. without a length check`
	db.Query("SELECT ?ident...", idents) // want `slice "idents" passed to \?ident\.\.\. without a length check`
}

func checkGuardedBailout(db *dali.DB, items []int) {
	if len(items) == 0 {
		return
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedBailoutPanic(db *dali.DB, items []int) {
	if len(items) == 0 {
		panic("unreachable")
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedBailoutOr(db *dali.DB, ok bool, items []int) {
	if len(items) == 0 || !ok {
		return
	}
	db.Query("SELECT ?...", items) // OK
}

func checkUnguardedBailoutAnd(db *dali.DB, ok bool, items []int) {
	if len(items) == 0 && !ok {
		return
	}
	db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
}

func checkGuardedBailoutLogFatal(db *dali.DB, items []int) {
	if len(items) == 0 {
		log.Fatal("no items")
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedBailoutOsExit(db *dali.DB, items []int) {
	if len(items) == 0 {
		os.Exit(1)
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedBailoutTestingSkip(t *testing.T, db *dali.DB, items []int) {
	if len(items) == 0 {
		t.Skip("no items")
	}
	db.Query("SELECT ?...", items) // OK
}

func checkUnguardedBailoutLogPrint(db *dali.DB, items []int) {
	if len(items) == 0 {
		log.Print("no items")
	}
	db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
}

func checkGuardedPositive(db *dali.DB, items []int) {
	if len(items) > 0 {
		db.Query("SELECT ?...", items) // OK
	}
}

func checkUnguardedPositiveElse(db *dali.DB, items []int) {
	if len(items) > 0 {
	} else {
		db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
	}
	if len(items) > 0 {
	} else if true {
		db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
	}
}

func checkGuardedNotEqual(db *dali.DB, items []int) {
	if len(items) != 0 {
		db.Query("SELECT ?...", items) // OK
	}
}

func checkGuardedDefault(db *dali.DB, items []int64) {
	if len(items) == 0 {
		items = []int64{-1}
	}
	db.Query("SELECT ?...", items) // OK
}

func checkGuardedCompound(db *dali.DB, items []int) {
	var id *int
	if id != nil && len(items) > 0 {
		db.Query("SELECT ?...", items) // OK
	}
	_ = id
}

func checkGuardedLessThanOne(db *dali.DB, items []int) {
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

func checkGuardedInClosure(db *dali.DB, items []int) {
	if len(items) == 0 {
		return
	}
	f := func() {
		db.Query("SELECT ?...", items) // OK
	}
	f()
}

func checkGuardedTransitiveRange(db *dali.DB, items []int) {
	if len(items) == 0 {
		return
	}
	var ids []int64
	for _, i := range items {
		ids = append(ids, int64(i))
	}
	db.Query("SELECT ?...", ids) // OK
}

func checkGuardedTransitiveRangeCompact(db *dali.DB, items []int) {
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

func checkGuardedTransitiveRangeNestedBlock(db *dali.DB, items []int) {
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

func checkUnguardedTransitiveRange(db *dali.DB, items []int) {
	var ids []int64
	for _, i := range items {
		ids = append(ids, int64(i))
	}
	db.Query("SELECT ?...", ids) // want `slice "ids" passed to \?\.\.\. without a length check`
}

func checkUnguardedTransitiveRangeSwitchContinue(db *dali.DB, items []int) {
	if len(items) == 0 {
		return
	}
	var ids []int64
	for _, i := range items {
		switch {
		case i < 0:
			continue
		}
		ids = append(ids, int64(i))
	}
	db.Query("SELECT ?...", ids) // want `slice "ids" passed to \?\.\.\. without a length check`
}

func checkGuardedTransitiveRangeSwitchBreak(db *dali.DB, items []int) {
	if len(items) == 0 {
		return
	}
	var ids []int64
	for _, i := range items {
		switch {
		case i < 0:
			break
		}
		ids = append(ids, int64(i))
	}
	db.Query("SELECT ?...", ids) // OK: break leaves only the switch
}

func checkGuardedTransitiveRangeLabeledSwitchBreak(db *dali.DB, items []int) {
	if len(items) == 0 {
		return
	}
	var ids []int64
	for _, i := range items {
	L:
		switch {
		case i < 0:
			break L
		}
		ids = append(ids, int64(i))
	}
	db.Query("SELECT ?...", ids) // OK: break L leaves only the switch
}

func checkGuardedMakeLen(db *dali.DB, items []int) {
	if len(items) == 0 {
		return
	}
	ids := make([]int64, len(items))
	for i, v := range items {
		ids[i] = int64(v)
	}
	db.Query("SELECT ?...", ids) // OK
}

func checkUnguardedMakeLen(db *dali.DB, items []int) {
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

type PtrValuer struct{}

func (*PtrValuer) Value() (driver.Value, error) { return nil, nil }

type PtrMarshaler struct{}

func (*PtrMarshaler) MarshalSQL(t dali.Translator) (string, error) { return "1", nil }

func checkPointerReceiverMethods(db *dali.DB) {
	db.Query("SELECT ?", &PtrValuer{})       // OK
	db.Query("SELECT ?", PtrValuer{})        // want `\? does not accept structs`
	db.Query("SELECT ?sql", &PtrMarshaler{}) // OK
	db.Query("SELECT ?sql", PtrMarshaler{})  // want `\?sql requires a string or dali\.Marshaler, got a\.PtrMarshaler`
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

type wrapper struct{ db *dali.DB }

func (w *wrapper) Query(sql string, args ...any) *dali.Query {
	return w.db.Query(sql, args...) // OK: forwarded
}

func (w *wrapper) QueryWithContext(ctx context.Context, sql string, args ...any) *dali.Query {
	return w.db.QueryWithContext(ctx, sql, args...) // OK: forwarded
}

func (w *wrapper) Count(table string) *dali.Query {
	q := "SELECT COUNT(*) FROM " + table
	return w.db.Query(q) // want `DB\.Query requires a constant query string`
}

func (w *wrapper) Deferred(sql string) func() *dali.Query {
	return func() *dali.Query {
		return w.db.Query(sql) // want `DB\.Query requires a constant query string`
	}
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

func checkGuardedDerivedViaMapPopulation(db *dali.DB, items []int) {
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

func checkGuardedDerivedViaMapPopulationClosure(db *dali.DB, items []int) {
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

func checkMethodExpression(db *dali.DB, q *dali.Query) {
	(*dali.DB).Query(db, "SELECT 1")       // OK
	(*dali.DB).Query(db, "SELECT ?, ?", 1) // want `DB\.Query has 2 placeholder\(s\) but 1 arg\(s\)`
	var ids []int
	(*dali.Query).ScanAllRows(q, &ids) // OK
}

func checkGeneric[T any, S ~string](db *dali.DB, rows []T, names []S) {
	if len(rows) == 0 || len(names) == 0 {
		return
	}
	db.Query("INSERT ?values...", rows) // OK: T is unknown
	db.Query("SELECT ?ident...", names) // OK: S is unknown
}

func checkGenericPointer[T any](db *dali.DB, rows []*T) {
	if len(rows) == 0 {
		return
	}
	db.Query("INSERT ?values...", rows) // OK: T is unknown
}

func checkUnguardedSelector(db *dali.DB) {
	var v struct{ IDs []int }
	db.Query("SELECT ?...", v.IDs) // want `slice "v.IDs" passed to \?\.\.\. without a length check`
}

func checkGuardedSelector(db *dali.DB) {
	var v struct{ IDs []int }
	if len(v.IDs) == 0 {
		return
	}
	db.Query("SELECT ?...", v.IDs) // OK
}

func checkGuardedTransitiveSelector(db *dali.DB, items []int) {
	if len(items) == 0 {
		return
	}
	var v struct{ IDs []int }
	for _, i := range items {
		v.IDs = append(v.IDs, i)
	}
	db.Query("SELECT ?...", v.IDs) // OK
}

func checkUnguardedDerivedCall(db *dali.DB, m map[int]bool) {
	db.Query("SELECT ?...", slices.Sorted(maps.Keys(m))) // want `slice "slices.Sorted\(maps.Keys\(m\)\)" passed to \?\.\.\. without a length check`
}

func checkGuardedDerivedCall(db *dali.DB, v struct{ M map[int]bool }) {
	if len(v.M) > 0 {
		db.Query("SELECT ?...", slices.Sorted(maps.Keys(v.M))) // OK
	}
}

func checkUntraceableCall(db *dali.DB) {
	ids, _ := makeItems()
	db.Query("SELECT ?...", slices.Clip(ids[:1])) // OK: cannot be checked without storing it
}

func checkSelectorAssignedFromGuarded(db *dali.DB, m map[int]bool, items []int) {
	if len(m) == 0 || len(items) == 0 {
		return
	}
	var v struct{ IDs, Pos []int }
	v.IDs = slices.Sorted(maps.Keys(m))
	db.Query("SELECT ?...", v.IDs) // OK
	v.Pos = make([]int, len(items))
	db.Query("SELECT ?...", v.Pos) // OK
}

func checkElseOfZeroGuard(db *dali.DB, items []int, ok bool) {
	if len(items) == 0 || !ok {
		db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
	} else {
		db.Query("SELECT ?...", items) // OK
	}
	if len(items) == 0 && ok {
		return
	} else {
		db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
	}
}

type skipper struct{}

func (skipper) Skip() {}

func checkCustomSkip(db *dali.DB, items []int, s skipper) {
	if len(items) == 0 {
		s.Skip()
	}
	db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
}

func checkIgnoreDirective(db *dali.DB, items []int) {
	db.Query("SELECT ?...", items) //dalivet:ignore
	db.Query("SELECT ?...", items) //dalivet:ignore callers check the length
	//dalivet:ignore callers check the length
	db.Query("SELECT ?...", items)
	db.Query("SELECT ?", items) //dalivet:ignore

	db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
	//dalivet:ignored
	db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
	db.Query("SELECT 1")           //dalivet:ignore
	db.Query("SELECT ?...", items) // want `slice "items" passed to \?\.\.\. without a length check`
}

func checkNonEmptyLiteral(db *dali.DB, id int64, items []int64) {
	ids := []int64{id}
	db.Query("SELECT ?...", ids) // OK

	var names = []string{"a", "b"}
	db.Query("SELECT ?ident...", names) // OK

	more := []int64{id}
	more = append(more, items...)
	more = slices.Compact(more)
	db.Query("SELECT ?...", more) // OK

	empty := []int64{}
	db.Query("SELECT ?...", empty) // want `slice "empty" passed to \?\.\.\. without a length check`

	spread := []int64{id}
	spread = items
	db.Query("SELECT ?...", spread) // want `slice "spread" passed to \?\.\.\. without a length check`

	var unset []int64
	db.Query("SELECT ?...", unset) // want `slice "unset" passed to \?\.\.\. without a length check`
}

var (
	defaultStates = map[string]bool{"active": true}
	defaultIDs    = []int64{1, 2}
	noIDs         = []int64{}
)

func checkNonEmptyPkgVar(db *dali.DB) {
	db.Query("SELECT ?...", slices.Sorted(maps.Keys(defaultStates))) // OK
	db.Query("SELECT ?...", defaultIDs)                              // OK
	ids := slices.Clone(defaultIDs)
	db.Query("SELECT ?...", ids)   // OK
	db.Query("SELECT ?...", noIDs) // want `slice "noIDs" passed to \?\.\.\. without a length check`

	defaultIDs := []int64{}
	db.Query("SELECT ?...", defaultIDs) // want `slice "defaultIDs" passed to \?\.\.\. without a length check`
}

func checkRepeatedSlice(db *dali.DB, items, ids []int) {
	db.Query("SELECT ?... UNION SELECT ?... UNION SELECT ?...",
		items, // want `slice "items" passed to \?\.\.\. without a length check`
		ids,   // want `slice "ids" passed to \?\.\.\. without a length check`
		items, // OK: reported above
	)
}
