// Package dalivet provides a go/analysis analyzer that catches common
// dali usage mistakes at compile time.
package dalivet

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const daliPkg = "github.com/mibk/dali"

// Analyzer is the dalivet analyzer for use with go/analysis.
var Analyzer = &analysis.Analyzer{
	Name:     "dalivet",
	Doc:      "checks for common dali usage mistakes",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

type checker struct {
	pass     *analysis.Pass
	timeType types.Type // time.Time, lazily resolved
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	c := &checker{pass: pass}

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	insp.WithStack(nodeFilter, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		call := n.(*ast.CallExpr)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		obj := pass.TypesInfo.ObjectOf(sel.Sel)
		fn, ok := obj.(*types.Func)
		if !ok {
			return true
		}

		recv := fn.Type().(*types.Signature).Recv()
		if recv == nil {
			return true
		}

		recvType := derefType(recv.Type())
		named, ok := recvType.(*types.Named)
		if !ok {
			return true
		}
		namedObj := named.Obj()
		if namedObj.Pkg() == nil || namedObj.Pkg().Path() != daliPkg {
			return true
		}

		typeName := namedObj.Name()
		methodName := sel.Sel.Name

		switch typeName {
		case "Query":
			c.checkQueryMethods(call, methodName)
		case "DB", "Tx":
			c.checkDBTxMethods(call, methodName, typeName, stack)
		}
		return true
	})

	return nil, nil
}

// checkQueryMethods handles Check 1: One/All/ScanAllRows argument types.
func (c *checker) checkQueryMethods(call *ast.CallExpr, method string) {
	switch method {
	case "One":
		if len(call.Args) != 1 {
			return
		}
		c.checkOneArg(call.Args[0])
	case "All":
		if len(call.Args) != 1 {
			return
		}
		c.checkAllArg(call.Args[0])
	case "ScanAllRows":
		for _, arg := range call.Args {
			c.checkScanAllRowsArg(arg)
		}
	}
}

// checkOneArg checks that One's argument is *Struct.
func (c *checker) checkOneArg(arg ast.Expr) {
	t := c.pass.TypesInfo.TypeOf(arg)
	if skipTypeCheck(t) {
		return
	}
	ptr, ok := deref(t)
	if !ok {
		c.pass.Reportf(arg.Pos(), "One requires a pointer to a struct, got %s", t)
		return
	}
	if !skipTypeCheck(ptr) && !isStruct(ptr) {
		c.pass.Reportf(arg.Pos(), "One requires a pointer to a struct, got %s", t)
	}
}

// checkAllArg checks that All's argument is *[]Struct or *[]*Struct.
func (c *checker) checkAllArg(arg ast.Expr) {
	t := c.pass.TypesInfo.TypeOf(arg)
	if skipTypeCheck(t) {
		return
	}
	ptr, ok := deref(t)
	if !ok {
		c.pass.Reportf(arg.Pos(), "All requires a pointer to a slice of structs, got %s", t)
		return
	}
	sl, ok := ptr.Underlying().(*types.Slice)
	if !ok {
		if !skipTypeCheck(ptr) {
			c.pass.Reportf(arg.Pos(), "All requires a pointer to a slice of structs, got %s", t)
		}
		return
	}
	elem := sl.Elem()
	if p, isPtr := deref(elem); isPtr {
		elem = p
		// Check for **Struct.
		if _, isPtr2 := deref(elem); isPtr2 {
			c.pass.Reportf(arg.Pos(), "All does not allow pointer to pointer as slice element, got %s", t)
			return
		}
	}
	if !skipTypeCheck(elem) && !isStruct(elem) {
		c.pass.Reportf(arg.Pos(), "All requires a pointer to a slice of structs, got %s", t)
	}
}

// checkScanAllRowsArg checks that each argument is *[]T.
func (c *checker) checkScanAllRowsArg(arg ast.Expr) {
	t := c.pass.TypesInfo.TypeOf(arg)
	if skipTypeCheck(t) {
		return
	}
	ptr, ok := deref(t)
	if !ok {
		c.pass.Reportf(arg.Pos(), "ScanAllRows requires a pointer to a slice, got %s", t)
		return
	}
	if _, ok := ptr.Underlying().(*types.Slice); !ok {
		if !skipTypeCheck(ptr) {
			c.pass.Reportf(arg.Pos(), "ScanAllRows requires a pointer to a slice, got %s", t)
		}
	}
}

// checkDBTxMethods handles Checks 2-4 for Query/Prepare methods.
func (c *checker) checkDBTxMethods(call *ast.CallExpr, method, typeName string, stack []ast.Node) {
	var isPrepare bool
	var queryArgIdx int
	switch method {
	case "Query":
		queryArgIdx = 0
	case "QueryWithContext":
		queryArgIdx = 1
	case "Prepare":
		isPrepare = true
		queryArgIdx = 0
	case "PrepareContext":
		isPrepare = true
		queryArgIdx = 1
	default:
		return
	}

	if queryArgIdx >= len(call.Args) {
		return
	}

	queryExpr := call.Args[queryArgIdx]
	tv, ok := c.pass.TypesInfo.Types[queryExpr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		c.pass.Reportf(queryExpr.Pos(), "%s.%s requires a constant query string", typeName, method)
		return
	}
	query := constant.StringVal(tv.Value)

	phs := parsePlaceholders(query)

	// Check 4: invalid placeholders and prepare restrictions.
	for _, ph := range phs {
		if !isValidPlaceholder(ph.Type) {
			c.pass.Reportf(queryExpr.Pos(), "unknown placeholder ?%s", ph.Type)
			return
		}
		if isPrepare {
			name := "?" + ph.Type
			if ph.Expand {
				name += "..."
			}
			switch {
			case ph.Type == "" && ph.Expand: // ?...
				c.pass.Reportf(queryExpr.Pos(), "%s cannot be used in prepared statements", name)
				return
			case ph.Type == "values":
				c.pass.Reportf(queryExpr.Pos(), "%s cannot be used in prepared statements", name)
				return
			case ph.Type == "set":
				c.pass.Reportf(queryExpr.Pos(), "%s cannot be used in prepared statements", name)
				return
			}
		}
	}

	// Check 2: arg count.
	var wantArgs int
	for _, ph := range phs {
		if isPrepare && ph.Type == "" && !ph.Expand {
			continue
		}
		wantArgs++
	}

	argsStart := queryArgIdx + 1
	gotArgs := len(call.Args) - argsStart
	if call.Ellipsis.IsValid() {
		return
	}
	if gotArgs != wantArgs {
		c.pass.Reportf(call.Pos(), "%s.%s has %d placeholder(s) but %d arg(s)",
			typeName, method, wantArgs, gotArgs)
		return
	}

	// Check 3: placeholder type checking.
	argIdx := argsStart
	for _, ph := range phs {
		if isPrepare && ph.Type == "" && !ph.Expand {
			continue
		}
		if argIdx >= len(call.Args) {
			break
		}
		arg := call.Args[argIdx]
		if ph.Expand {
			switch a := arg.(type) {
			case *ast.CompositeLit:
				if len(a.Elts) == 0 {
					c.pass.Reportf(arg.Pos(), "empty slice passed to ?%s...", ph.Type)
				}
			case *ast.Ident:
				if !isSliceGuarded(stack, a) {
					c.pass.Reportf(arg.Pos(),
						"slice %q passed to ?%s... without a length check", a.Name, ph.Type)
				}
			}
		}
		argType := c.pass.TypesInfo.TypeOf(arg)
		if !isInterface(argType) {
			c.checkPlaceholderType(arg, ph, argType)
		}
		argIdx++
	}
}

func isValidPlaceholder(typ string) bool {
	switch typ {
	case "", "ident", "values", "set", "sql":
		return true
	}
	return false
}

func (c *checker) checkPlaceholderType(arg ast.Expr, ph Placeholder, argType types.Type) {
	if ph.Expand {
		switch ph.Type {
		case "": // ?...
			if !isSlice(argType) {
				c.pass.Reportf(arg.Pos(), "?... requires a slice, got %s", argType)
			}
		case "ident": // ?ident...
			if !isSliceOfString(argType) {
				c.pass.Reportf(arg.Pos(), "?ident... requires []string, got %s", argType)
			}
		case "values": // ?values...
			if !isSliceOfStructsOrMaps(argType) {
				c.pass.Reportf(arg.Pos(), "?values... requires a slice of structs or dali.Map, got %s", argType)
			}
		}
		return
	}

	switch ph.Type {
	case "": // ?
		c.checkScalarArg(arg, argType)
	case "ident": // ?ident
		if !isStringType(argType) {
			c.pass.Reportf(arg.Pos(), "?ident requires a string, got %s", argType)
		}
	case "values": // ?values
		c.checkStructOrMap(arg, argType, "?values")
	case "set": // ?set
		c.checkStructOrMap(arg, argType, "?set")
	case "sql": // ?sql
		if !isStringType(argType) && !implementsMarshaler(argType) {
			c.pass.Reportf(arg.Pos(), "?sql requires a string or dali.Marshaler, got %s", argType)
		}
	}
}

func (c *checker) checkScalarArg(arg ast.Expr, t types.Type) {
	und := t.Underlying()

	if _, ok := und.(*types.Pointer); ok {
		return
	}

	if implementsValuer(t) {
		return
	}

	switch u := und.(type) {
	case *types.Basic:
		return
	case *types.Slice:
		if elem, ok := u.Elem().(*types.Basic); ok && elem.Kind() == types.Byte {
			return
		}
		c.pass.Reportf(arg.Pos(), "? does not accept slices (except []byte), got %s", t)
	case *types.Struct:
		if c.isTimeConvertible(t) {
			return
		}
		c.pass.Reportf(arg.Pos(), "? does not accept structs (except time.Time-convertible or driver.Valuer), got %s", t)
	default:
		c.pass.Reportf(arg.Pos(), "? does not accept %s", t)
	}
}

func (c *checker) checkStructOrMap(arg ast.Expr, t types.Type, placeholder string) {
	orig := t
	if p, ok := deref(t); ok {
		t = p
	}
	if isStruct(t) {
		return
	}
	if isDaliMap(t) {
		return
	}
	c.pass.Reportf(arg.Pos(), "%s requires a struct or dali.Map, got %s", placeholder, orig)
}

func (c *checker) isTimeConvertible(t types.Type) bool {
	if c.timeType == nil {
		c.timeType = c.findTimeType()
		if c.timeType == nil {
			// time package not imported; can't verify. Be lenient.
			return true
		}
	}
	return types.ConvertibleTo(t, c.timeType)
}

func (c *checker) findTimeType() types.Type {
	for _, pkg := range c.pass.Pkg.Imports() {
		if pkg.Path() == "time" {
			obj := pkg.Scope().Lookup("Time")
			if obj != nil {
				return obj.Type()
			}
		}
	}
	return nil
}

// --- type helpers ---

func deref(t types.Type) (types.Type, bool) {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem(), true
	}
	if p, ok := t.Underlying().(*types.Pointer); ok {
		return p.Elem(), true
	}
	return t, false
}

func derefType(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

func isInterface(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

func isTypeParam(t types.Type) bool {
	_, ok := t.(*types.TypeParam)
	return ok
}

// skipTypeCheck returns true for interfaces and type parameters,
// where the concrete type isn't known statically.
func skipTypeCheck(t types.Type) bool {
	return isInterface(t) || isTypeParam(t)
}

func isStruct(t types.Type) bool {
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

func isSlice(t types.Type) bool {
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

func isStringType(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

func isSliceOfString(t types.Type) bool {
	sl, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	return isStringType(sl.Elem())
}

func isDaliMap(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == daliPkg && obj.Name() == "Map"
}

func isSliceOfStructsOrMaps(t types.Type) bool {
	sl, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	elem := sl.Elem()
	if p, ok := elem.(*types.Pointer); ok {
		elem = p.Elem()
	}
	return isStruct(elem) || isDaliMap(elem)
}

func implementsMarshaler(t types.Type) bool {
	return hasMethod(t, "MarshalSQL", 1, 2)
}

func implementsValuer(t types.Type) bool {
	return hasMethod(t, "Value", 0, 2)
}

// nonEmptyPreserving lists stdlib functions that preserve the non-emptiness
// of their first argument: if the input is non-empty, the output is non-empty.
var nonEmptyPreserving = map[string]map[string]bool{
	"slices": {
		"Sorted": true, "SortedFunc": true, "SortedStableFunc": true,
		"Compact": true, "CompactFunc": true, "Clip": true,
		"Clone": true, "Collect": true,
	},
	"maps": {"Keys": true, "Values": true, "Collect": true},
}

// traceCallSource peels nested calls to known non-empty-preserving stdlib
// functions and returns the innermost identifier name. For example, given
// slices.Sorted(maps.Keys(m)) it returns "m".
func traceCallSource(expr ast.Expr) string {
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			break
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || len(call.Args) == 0 {
			return ""
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return ""
		}
		funcs, ok := nonEmptyPreserving[pkg.Name]
		if !ok || !funcs[sel.Sel.Name] {
			return ""
		}
		expr = call.Args[0]
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// traceSliceSource scans stmts for an assignment to name whose RHS traces
// (via traceCallSource) to a source identifier. Returns that identifier's
// name or "".
func traceSliceSource(stmts []ast.Stmt, name string) string {
	for _, stmt := range stmts {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != name {
			continue
		}
		if src := traceCallSource(assign.Rhs[0]); src != "" {
			return src
		}
	}
	return ""
}

// isSliceGuarded reports whether ident is guarded by a length check in the
// surrounding code. It walks enclosing blocks outward (including through
// closure boundaries) and recognizes these patterns:
//
//   - Bail-out:    if len(x) == 0 { return } preceding the call
//   - Default:     if len(x) == 0 { x = fallback } preceding the call
//   - Positive:    the call is inside if len(x) > 0 { ... }
//   - Transitive:  x built via append inside for range Y, where Y is guarded
//   - Make(len):   x := make([]T, len(Y)), where Y is guarded
//   - Derived:     x := slices.Sorted(maps.Keys(m)) inside if len(m) > 0
//
// traceNonEmptySource scans stmts (up to but not including before) for a
// statement that establishes name's non-emptiness from some source variable.
// Returns that source's name or "".
//
// Recognized patterns:
//   - name := f(g(h(x))) where f,g,h are non-empty-preserving stdlib calls
//   - for _, v := range X { name[k] = v } (map population)
//   - for _, v := range X { name = append(name, ...) } (slice population)
func traceNonEmptySource(stmts []ast.Stmt, before ast.Node, name string) string {
	for _, stmt := range stmts {
		if stmt == before {
			break
		}
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
				continue
			}
			lhs, ok := s.Lhs[0].(*ast.Ident)
			if !ok || lhs.Name != name {
				continue
			}
			if src := traceCallSource(s.Rhs[0]); src != "" {
				return src
			}
		case *ast.RangeStmt:
			src := exprName(s.X)
			if src == "" {
				continue
			}
			if bodyAppendsTo(s.Body, name) || bodyWritesMapEntry(s.Body, name) {
				return src
			}
		}
	}
	return ""
}

// bodyWritesMapEntry reports whether body unconditionally contains a
// statement of the form name[k] = v (map index assignment). Like
// bodyAppendsTo, it returns false if any earlier statement can skip
// the iteration.
func bodyWritesMapEntry(body *ast.BlockStmt, name string) bool {
	for _, stmt := range body.List {
		if stmtCanExit(stmt) {
			return false
		}
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 {
			continue
		}
		idx, ok := assign.Lhs[0].(*ast.IndexExpr)
		if !ok {
			continue
		}
		id, ok := idx.X.(*ast.Ident)
		if ok && id.Name == name {
			return true
		}
	}
	return false
}

func isSliceGuarded(stack []ast.Node, ident *ast.Ident) bool {
	return isSliceNameGuarded(stack, ident.Name, nil)
}

func isSliceNameGuarded(stack []ast.Node, name string, visited map[string]bool) bool {
	// Pattern B: call inside if len(x) > 0 { ... } (positive guard).
	// The len check may be one conjunct in a && chain.
	// Extended: if the direct name doesn't match, trace the source of
	// name from the if body and re-check.
	for i := len(stack) - 1; i >= 0; i-- {
		ifStmt, ok := stack[i].(*ast.IfStmt)
		if !ok {
			continue
		}
		if condHasPositiveLenGuard(ifStmt.Cond, name) {
			return true
		}
		if src := traceSliceSource(ifStmt.Body.List, name); src != "" {
			if condHasPositiveLenGuard(ifStmt.Cond, src) {
				return true
			}
		}
	}

	// Walk enclosing BlockStmts from innermost outward, checking each
	// for guard patterns preceding the call (or the statement that
	// contains the call in that block's scope).
	for i := len(stack) - 1; i >= 0; i-- {
		block, ok := stack[i].(*ast.BlockStmt)
		if !ok {
			continue
		}
		if i+1 >= len(stack) {
			continue
		}
		callStmt := stack[i+1]
		if blockGuardsSlice(block, callStmt, name) {
			return true
		}
	}

	// Trace non-empty source: if name is derived (via non-empty-preserving
	// calls or for-range population) from some other variable, recursively
	// check whether that source is guarded. This uses the full stack so it
	// works across closure boundaries.
	for i := len(stack) - 1; i >= 0; i-- {
		block, ok := stack[i].(*ast.BlockStmt)
		if !ok {
			continue
		}
		if i+1 >= len(stack) {
			continue
		}
		callStmt := stack[i+1]
		if src := traceNonEmptySource(block.List, callStmt, name); src != "" {
			if visited == nil {
				visited = map[string]bool{name: true}
			}
			if !visited[src] {
				visited[src] = true
				if isSliceNameGuarded(stack, src, visited) {
					return true
				}
			}
		}
	}
	return false
}

// blockGuardsSlice checks whether block contains a guard for name before
// callStmt. It checks Pattern A (bail-out/default), Pattern C (for range +
// append), and Pattern D (make with len).
func blockGuardsSlice(block *ast.BlockStmt, callStmt ast.Node, name string) bool {
	// Pattern A: bail-out or default-value preceding the call.
	if hasBailoutGuard(block.List, callStmt, name) {
		return true
	}
	// Pattern C: name is appended to inside for range X, and X has a
	// bail-out guard earlier in the block.
	// Pattern D: name := make([]T, len(X)) and X has a bail-out guard.
	for _, stmt := range block.List {
		if stmt == callStmt {
			break
		}
		switch s := stmt.(type) {
		case *ast.RangeStmt:
			src := exprName(s.X)
			if src == "" {
				continue
			}
			if !bodyAppendsTo(s.Body, name) {
				continue
			}
			if hasBailoutGuard(block.List, s, src) {
				return true
			}
		case *ast.AssignStmt:
			if src := makeLenSource(s, name); src != "" {
				if hasBailoutGuard(block.List, s, src) {
					return true
				}
			}
		}
	}
	return false
}

// hasBailoutGuard reports whether stmts (up to, not including, before)
// contains an if statement whose condition is a zero-length guard on name
// and whose body terminates or assigns a default value to name.
func hasBailoutGuard(stmts []ast.Stmt, before ast.Node, name string) bool {
	for _, stmt := range stmts {
		if stmt == before {
			break
		}
		ifStmt, ok := stmt.(*ast.IfStmt)
		if !ok {
			continue
		}
		if ifChainGuardsSlice(ifStmt, name) {
			return true
		}
	}
	return false
}

// ifChainGuardsSlice walks an if / else-if chain looking for a
// zero-length guard on name.  An else-if branch counts as a valid
// guard when every preceding branch in the chain terminates, because
// reaching code after the chain implies the guard condition was false.
func ifChainGuardsSlice(ifStmt *ast.IfStmt, name string) bool {
	for {
		op, val, ok := matchLenCheck(ifStmt.Cond, name)
		if ok && isZeroGuard(op, val) && (blockTerminates(ifStmt.Body) || blockAssigns(ifStmt.Body, name)) {
			return true
		}
		// Walk into the else-if chain only when the current body
		// terminates — otherwise execution can fall through without
		// the length ever being tested.
		elseIf, ok := ifStmt.Else.(*ast.IfStmt)
		if !ok || !blockTerminates(ifStmt.Body) {
			return false
		}
		ifStmt = elseIf
	}
}

// bodyAppendsTo reports whether body unconditionally contains a top-level
// assignment of the form name = append(name, ...). It returns false if any
// earlier statement can skip the iteration (continue/break), since the append
// would then be conditional and the result could be empty.
func bodyAppendsTo(body *ast.BlockStmt, name string) bool {
	for _, stmt := range body.List {
		if stmtCanExit(stmt) {
			return false
		}
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != name {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || fn.Name != "append" {
			continue
		}
		if len(call.Args) < 1 {
			continue
		}
		first, ok := call.Args[0].(*ast.Ident)
		if !ok || first.Name != name {
			continue
		}
		return true
	}
	return false
}

// stmtCanExit reports whether stmt contains a continue or break that
// could skip subsequent statements in a for body. Return statements
// are NOT counted because if a return fires the slice is never used,
// so the non-empty guarantee still holds for reachable code.
func stmtCanExit(stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case *ast.BranchStmt:
		return s.Tok == token.CONTINUE || s.Tok == token.BREAK
	case *ast.IfStmt:
		return blockCanExit(s.Body) || (s.Else != nil && stmtCanExit(s.Else))
	}
	return false
}

func blockCanExit(block *ast.BlockStmt) bool {
	for _, stmt := range block.List {
		if stmtCanExit(stmt) {
			return true
		}
	}
	return false
}

// makeLenSource checks whether assign is of the form name = make([]T, len(X))
// or name := make([]T, len(X)), and returns X's name if so. Otherwise "".
func makeLenSource(assign *ast.AssignStmt, name string) string {
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return ""
	}
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || lhs.Name != name {
		return ""
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return ""
	}
	fn, ok := call.Fun.(*ast.Ident)
	if !ok || fn.Name != "make" {
		return ""
	}
	lenCall, ok := call.Args[1].(*ast.CallExpr)
	if !ok {
		return ""
	}
	lenFn, ok := lenCall.Fun.(*ast.Ident)
	if !ok || lenFn.Name != "len" || len(lenCall.Args) != 1 {
		return ""
	}
	src, ok := lenCall.Args[0].(*ast.Ident)
	if !ok {
		return ""
	}
	return src.Name
}

// matchLenCheck matches expressions of the form len(name) <op> <int> or
// <int> <op> len(name), returning the operator (normalized so len is on the
// left) and the integer value.
func matchLenCheck(expr ast.Expr, name string) (token.Token, int64, bool) {
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return 0, 0, false
	}

	lenOnLeft := isLenCall(bin.X, name)
	lenOnRight := isLenCall(bin.Y, name)
	if !lenOnLeft && !lenOnRight {
		return 0, 0, false
	}

	var litSide ast.Expr
	op := bin.Op
	if lenOnLeft {
		litSide = bin.Y
	} else {
		litSide = bin.X
		// Normalize: flip operator so len is conceptually on the left.
		switch op {
		case token.LSS:
			op = token.GTR
		case token.GTR:
			op = token.LSS
		case token.LEQ:
			op = token.GEQ
		case token.GEQ:
			op = token.LEQ
		}
	}

	lit, ok := litSide.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, 0, false
	}
	val, err := strconv.ParseInt(lit.Value, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return op, val, true
}

// condHasPositiveLenGuard reports whether expr (or any conjunct in an &&
// chain) contains a positive length guard for name.
func condHasPositiveLenGuard(expr ast.Expr, name string) bool {
	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.LAND {
		return condHasPositiveLenGuard(bin.X, name) || condHasPositiveLenGuard(bin.Y, name)
	}
	op, val, ok := matchLenCheck(expr, name)
	if !ok {
		return false
	}
	return isPositiveGuard(op, val)
}

func isLenCall(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	fn, ok := call.Fun.(*ast.Ident)
	if !ok || fn.Name != "len" {
		return false
	}
	return exprName(call.Args[0]) == name
}

// exprName returns a string representation for simple name expressions:
// plain identifiers ("x") and one-level selectors ("v.Payments").
func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
	}
	return ""
}

// isZeroGuard returns true if the condition means "length is zero":
// len(x) == 0 or len(x) < 1.
func isZeroGuard(op token.Token, val int64) bool {
	return (op == token.EQL && val == 0) || (op == token.LSS && val == 1)
}

// isPositiveGuard returns true if the condition means "length is positive":
// len(x) > 0, len(x) != 0, or len(x) >= 1.
func isPositiveGuard(op token.Token, val int64) bool {
	return (op == token.GTR && val == 0) ||
		(op == token.NEQ && val == 0) ||
		(op == token.GEQ && val == 1)
}

// blockAssigns reports whether block contains an assignment to the named
// identifier (e.g. x = ...). This recognizes the "default value" pattern:
//
//	if len(x) == 0 { x = []int{-1} }
func blockAssigns(block *ast.BlockStmt, name string) bool {
	for _, stmt := range block.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.ASSIGN {
			continue
		}
		for _, lhs := range assign.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
				return true
			}
		}
	}
	return false
}

func blockTerminates(block *ast.BlockStmt) bool {
	if len(block.List) == 0 {
		return false
	}
	last := block.List[len(block.List)-1]
	switch s := last.(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "panic" {
				return true
			}
		}
	}
	return false
}

func hasMethod(t types.Type, name string, params, results int) bool {
	// Check both value and pointer receiver method sets.
	for _, ms := range []*types.MethodSet{
		types.NewMethodSet(t),
		types.NewMethodSet(types.NewPointer(t)),
	} {
		for i := 0; i < ms.Len(); i++ {
			sel := ms.At(i)
			if sel.Obj().Name() != name {
				continue
			}
			fn, ok := sel.Obj().(*types.Func)
			if !ok {
				continue
			}
			sig := fn.Type().(*types.Signature)
			if sig.Params().Len() == params && sig.Results().Len() == results {
				return true
			}
		}
	}
	return false
}
