package loneanalyzer

import (
	"strings"
	"testing"
)

func TestLetImmutabilityEnforcement(t *testing.T) {
	code := `
let max = 100
max = 200

var count = 0
count = 10
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}

	d := diags[0]
	if d.Rule != "type/immutable-reassignment" {
		t.Errorf("expected rule 'type/immutable-reassignment', got %q", d.Rule)
	}
	if !strings.Contains(d.Message, "cannot reassign immutable binding \"max\"") {
		t.Errorf("unexpected error message: %q", d.Message)
	}
}

func TestVarMutabilityAllowed(t *testing.T) {
	code := `
var total = 0
total = 10
total = total + 5
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestScopedConditionHeaderScope(t *testing.T) {
	code := `
fn test_scoped() void {
    if: [let scoped_var = 42] scoped_var > 0 {
        let inside = scoped_var
    }
    let leak = scoped_var
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// 'scoped_var' must be undefined outside the 'if' construct
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic (scope leak), got %d: %v", len(diags), diags)
	}

	d := diags[0]
	if d.Rule != "type/undefined-identifier" || !strings.Contains(d.Message, "scoped_var") {
		t.Errorf("expected undefined identifier 'scoped_var', got %v", d)
	}
}

func TestScopedConditionHeaderPackedIf(t *testing.T) {
	code := `
fn test_packed() void {
    if: [let x = 10, let y = 20] {
        x > 0 and y > 0 => {
            drop x + y
        },
        else => {
            drop 0
        }
    }
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestTrTraitNamingConvention(t *testing.T) {
	// 1. Valid Tr trait
	validCode := `
pub type TrWriter = trait {
    fn write(self, buf: []u8) i32
}
pub type Stdout = struct {
    fd: i32
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(validCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for valid TrWriter, got: %v", diags)
	}

	// 2. Trait without Tr prefix
	invalidTraitCode := `
pub type WriterTrait = trait {
    fn write(self) void
}
`
	analyzer2 := NewAnalyzer()
	_, diags2, err := analyzer2.AnalyzeSource([]byte(invalidTraitCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags2) != 1 || diags2[0].Rule != "convention/trait-naming" {
		t.Errorf("expected convention/trait-naming error, got: %v", diags2)
	}

	// 3. Struct with Tr prefix
	invalidStructCode := `
pub type TrMyStruct = struct {
    x: i32
}
`
	analyzer3 := NewAnalyzer()
	_, diags3, err := analyzer3.AnalyzeSource([]byte(invalidStructCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags3) != 1 || diags3[0].Rule != "convention/non-trait-naming" {
		t.Errorf("expected convention/non-trait-naming error, got: %v", diags3)
	}
}

func TestExplicitCastOperator(t *testing.T) {
	code := `
let i: i32 = 100
let f: f32 = i as f32
let w: i64 = i as i64
`
	analyzer := NewAnalyzer()
	prog, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for explicit 'as' casts, got: %v", diags)
	}

	fVar := prog.Decls[1].(*TVarDec)
	if !fVar.Type.Equals(TypeF32) {
		t.Errorf("expected type f32, got %q", fVar.Type.String())
	}
}

func TestImplicitNumericMismatchRejected(t *testing.T) {
	code := `
var f: f32 = 100
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic (type mismatch for implicit int to float), got %d: %v", len(diags), diags)
	}
	if diags[0].Rule != "type/mismatch" {
		t.Errorf("expected rule 'type/mismatch', got %q", diags[0].Rule)
	}
}

func TestEnumAndMatchAnalysis(t *testing.T) {
	code := `
type Shape = enum {
    Circle(radius: f32),
    Rect(width: f32, height: f32),
    Point,
}

fn area(s: Shape) f32 {
    return match s {
        Shape::Circle(r) => 3.14 * r * r,
        Shape::Rect(w, h) => w * h,
        Shape::Point => 0.0,
    }
}

fn test_construct() Shape {
    let c = Shape::Circle(10.5)
    let p = Shape::Point
    return c
}
`
	analyzer := NewAnalyzer()
	prog, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}

	enumDec, ok := prog.Decls[0].(*TEnumDec)
	if !ok {
		t.Fatalf("expected TEnumDec, got %#v", prog.Decls[0])
	}
	// Tag (4) + Rect payload (4+4) = 12 bytes
	if enumDec.Type.TotalSize != 12 {
		t.Errorf("expected Shape size 12, got %d", enumDec.Type.TotalSize)
	}
}

func TestNonExhaustiveMatchError(t *testing.T) {
	code := `
type Shape = enum {
    Circle(radius: f32),
    Rect(width: f32, height: f32),
    Point,
}

fn area(s: Shape) f32 {
    return match s {
        Shape::Circle(r) => 3.14 * r * r,
        Shape::Point => 0.0,
    }
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for missing Rect variant, got %d: %v", len(diags), diags)
	}
	if diags[0].Rule != "type/non-exhaustive-match" {
		t.Errorf("expected rule 'type/non-exhaustive-match', got %q", diags[0].Rule)
	}
	if !strings.Contains(diags[0].Message, "Rect") {
		t.Errorf("expected message to mention missing variant 'Rect', got %q", diags[0].Message)
	}
}

func TestDiscriminantIsAndHeaderDestructuring(t *testing.T) {
	code := `
type Shape = enum {
    Circle(radius: i32),
    Rect(width: i32, height: i32),
    Point,
}

fn check_shape(s: Shape) bool {
    let is_circ = s is Shape::Circle
    let not_pt = s is not Shape::Point

    if: [let Shape::Circle(r) = s] {
        return r > 0
    } else {
        return false
    }
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got: %v", diags)
	}
}

func TestMustAndTry(t *testing.T) {
	code := `
type Result = enum {
    Ok(val: i32),
    Err(code: i32),
}

fn step1(r: Result) Result {
    let val = try r
    let direct = must r
    return Result::Ok(val + direct)
}

fn bad_try(r: Result) i32 {
    let val = try r
    return val
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	// bad_try should trigger type/try-incompatible-return
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for bad_try return mismatch, got %d: %v", len(diags), diags)
	}
	if diags[0].Rule != "type/try-incompatible-return" {
		t.Errorf("expected rule 'type/try-incompatible-return', got %q", diags[0].Rule)
	}
}

func TestGenericsAndResultInfer(t *testing.T) {
	code := `
pub type Option<T> = enum {
    Some(val: T),
    None,
}

pub type Pair<A, B> = struct {
    first: A,
    second: B,
}

fn wrap_val<T>(val: T) Option<T> {
    return Option::Some(val)
}

fn get_pair() Pair<i32, string> {
    return Pair::{ .first: 10, .second: "hello" }
}

fn compute_inferred(a: i32) Result<i32, infer> {
    if a < 0 {
        return Result::Err(Error::Code(400))
    }
    return Result::Ok(a * 2)
}

fn call_inferred() Result<i32> {
    let x = try compute_inferred(21)
    return Result::Ok(x)
}
`
	analyzer := NewAnalyzer()
	prog, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if prog == nil {
		t.Fatalf("expected prog")
	}
}

func TestGenericConstraintsIsError(t *testing.T) {
	goodCode := `
pub type CustomErr = enum {
    Failed,
}

fn valid_default() Result<i32> {
    return Result::Ok(1)
}

fn valid_infer() Result<i32, infer> {
    return Result::Ok(2)
}

fn valid_custom() Result<i32, CustomErr> {
    return Result::Ok(3)
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(goodCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diags for valid Result types, got %d: %v", len(diags), diags)
	}

	badCode := `
fn invalid_err_type() Result<i32, string> {
    return Result::Ok(42)
}
`
	analyzer2 := NewAnalyzer()
	_, diags2, err := analyzer2.AnalyzeSource([]byte(badCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags2) == 0 {
		t.Fatalf("expected constraint violation diagnostic for Result<i32, string>")
	}
	hasViolation := false
	for _, d := range diags2 {
		if d.Rule == "type/constraint-violation" && strings.Contains(d.Message, "does not satisfy 'is Error'") {
			hasViolation = true
			break
		}
	}
	if !hasViolation {
		t.Errorf("expected diagnostic with rule 'type/constraint-violation' and message mentioning 'is Error', got: %v", diags2)
	}
}

func TestGenericConstraintsSatisfiesAndTraitMaterialization(t *testing.T) {
	code := `
pub type TrWriter = trait {
    fn write(self, msg: str) void
}

pub type Console = struct {
    id: int,
}

impl TrWriter on Console {
    fn write(self, msg: str) void {
        #print(msg)
    }
}

pub type Dummy = struct {
    x: int,
}

fn log_msg<T where { T satisfies TrWriter }>(writer: T, msg: str) void {
    let w = TrWriter::|T|
    w.write(writer, msg)
}

fn test_valid() void {
    let c = Console::{ .id: 1 }
    log_msg(c, "hello\n")
}

fn test_invalid() void {
    let d = Dummy::{ .x: 2 }
    log_msg(d, "fail\n")
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) == 0 {
		t.Fatalf("expected constraint violation for Dummy not implementing TrWriter")
	}
	hasViolation := false
	for _, d := range diags {
		if d.Rule == "type/constraint-violation" && strings.Contains(d.Message, "does not satisfy trait \"TrWriter\"") {
			hasViolation = true
			break
		}
	}
	if !hasViolation {
		t.Errorf("expected type/constraint-violation for Dummy, got: %v", diags)
	}
}

func TestStructEncapsulationAndPrivateFields(t *testing.T) {
	code := `
type Account = struct {
    id: int,
    _balance: int,

    pub fn __init__(id: int, deposit: int) Account {
        return Account::{
            .id: id,
            ._balance: deposit,
        }
    }

    pub fn get_balance(self) int {
        return self._balance
    }
}

fn test_valid() void {
    let acc = Account::(101, 500)
    let bal = acc.get_balance()
}

fn test_invalid_raw_construct() void {
    let bad = Account::{ .id: 102 }
}

fn test_invalid_private_access() void {
    let acc = Account::(103, 300)
    let secret = acc._balance
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	var hasRawConstructErr, hasPrivateAccessErr bool
	for _, d := range diags {
		if d.Rule == "type/raw-construct-private-fields" {
			hasRawConstructErr = true
		}
		if d.Rule == "type/private-field-access" {
			hasPrivateAccessErr = true
		}
	}

	if !hasRawConstructErr {
		t.Errorf("expected type/raw-construct-private-fields diagnostic, got: %v", diags)
	}
	if !hasPrivateAccessErr {
		t.Errorf("expected type/private-field-access diagnostic, got: %v", diags)
	}
}

func TestStructReadOnlyFieldEnforcement(t *testing.T) {
	code := `
type Point = struct {
    ro id: int,
    val: int,
}

fn test_ro() void {
    var p = Point::{ .id: 1, .val: 10 }
    p.val = 20
    p.id = 99
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	hasRoWriteErr := false
	for _, d := range diags {
		if d.Rule == "type/immutable-field-write" {
			hasRoWriteErr = true
			break
		}
	}

	if !hasRoWriteErr {
		t.Errorf("expected type/immutable-field-write diagnostic, got: %v", diags)
	}
}

func TestStructMissingInitForRoPrivateField(t *testing.T) {
	code := `
type SecretRecord = struct {
    ro _key: str,
    title: str,
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	hasMissingInitErr := false
	for _, d := range diags {
		if d.Rule == "type/missing-init-for-ro-private-field" {
			hasMissingInitErr = true
			break
		}
	}

	if !hasMissingInitErr {
		t.Errorf("expected type/missing-init-for-ro-private-field diagnostic, got: %v", diags)
	}
}

func TestStructUnresolvedConstructor(t *testing.T) {
	code := `
type PlainData = struct {
    x: int,
    y: int,
}

fn test_bad_constructor() void {
    let p = PlainData::(10, 20)
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	hasUnresolvedErr := false
	for _, d := range diags {
		if d.Rule == "type/unresolved-constructor" {
			hasUnresolvedErr = true
			break
		}
	}

	if !hasUnresolvedErr {
		t.Errorf("expected type/unresolved-constructor diagnostic, got: %v", diags)
	}
}

func TestRefVsPtrImmutabilityTracking(t *testing.T) {
	code := `
fn test_immutable_writes() void {
    let x = 42
    let r = &x
    *r = 99

    let p = &raw x
    *p = 100
    p[0] = 101

    var y = 10
    var r_mut = &y
    *r_mut = 20
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	var hasRefWriteErr, hasPtrWriteErr bool
	for _, d := range diags {
		if d.Rule == "type/immutable-reference-write" {
			hasRefWriteErr = true
		}
		if d.Rule == "type/immutable-pointer-write" {
			hasPtrWriteErr = true
		}
	}

	if !hasRefWriteErr {
		t.Errorf("expected type/immutable-reference-write diagnostic, got: %v", diags)
	}
	if !hasPtrWriteErr {
		t.Errorf("expected type/immutable-pointer-write diagnostic, got: %v", diags)
	}
}

func TestPointerIndexingAllowedAndRefForbidden(t *testing.T) {
	code := `
fn test_indexing(p: ptr<i32>, r: ref<i32>) void {
    let val_p = p[2]
    p[2] = 42

    let val_r = r[2]
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	hasRefIndexErr := false
	for _, d := range diags {
		if d.Rule == "type/invalid-index" && strings.Contains(d.Message, "cannot index managed reference") {
			hasRefIndexErr = true
			break
		}
	}

	if !hasRefIndexErr {
		t.Errorf("expected type/invalid-index diagnostic for ref, got: %v", diags)
	}
}

func TestGroupedDeclarations(t *testing.T) {
	code := `
fn add_points([p1, p2]: (int, int)) int {
    let (x1, y1) = p1
    let (x2, y2) = p2
    return x1 + y1 + x2 + y2
}

fn test_grouped_locals() int {
    var [top, bottom, left, right]: int = 0
    top = 10
    bottom = 20
    left = 30
    right = 40
    return top + bottom + left + right
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error diagnostic: %v", d)
		}
	}
}

func TestTuplesAndDestructuring(t *testing.T) {
	code := `
fn test_tuples() void {
    let pair = (10, 20)
    let a = pair.0
    let b = pair.1
    let bad_idx = pair.2

    let (x, y) = pair
    let (u, v, w) = pair
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	var hasOobErr, hasArityErr bool
	for _, d := range diags {
		if d.Rule == "type/tuple-index-out-of-bounds" {
			hasOobErr = true
		}
		if d.Rule == "type/tuple-arity-mismatch" {
			hasArityErr = true
		}
	}

	if !hasOobErr {
		t.Errorf("expected type/tuple-index-out-of-bounds, got: %v", diags)
	}
	if !hasArityErr {
		t.Errorf("expected type/tuple-arity-mismatch, got: %v", diags)
	}
}

func TestAnonymousFunctionsAndEscapingCapture(t *testing.T) {
	validCode := `
fn test_valid_anon() int {
    let square = fn(x: int) int {
        return x * x
    }
    return square(5)
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(validCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error diagnostic in valid anon fn: %v", d)
		}
	}

	badCaptureCode := `
fn test_bad_capture() void {
    var local_val = 42
    let bad_fn = fn() void {
        let r = &local_val
    }
}
`
	analyzer2 := NewAnalyzer()
	_, diags2, err := analyzer2.AnalyzeSource([]byte(badCaptureCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	hasCaptureErr := false
	for _, d := range diags2 {
		if d.Rule == "closure/escaping-reference-capture" {
			hasCaptureErr = true
			break
		}
	}

	if !hasCaptureErr {
		t.Errorf("expected closure/escaping-reference-capture diagnostic, got: %v", diags2)
	}
}

func TestInterfaceNamingConventions(t *testing.T) {
	codeBad1 := `
type Point = interface {
    x: int,
    y: int,
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(codeBad1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasNamingErr := false
	for _, d := range diags {
		if d.Rule == "naming/interface" {
			hasNamingErr = true
		}
	}
	if !hasNamingErr {
		t.Errorf("expected naming/interface diagnostic for Point, got: %v", diags)
	}

	codeBad2 := `
type IfcFake = struct {
    x: int,
}
`
	_, diags2, err := analyzer.AnalyzeSource([]byte(codeBad2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasNonIfcErr := false
	for _, d := range diags2 {
		if d.Rule == "naming/interface" {
			hasNonIfcErr = true
		}
	}
	if !hasNonIfcErr {
		t.Errorf("expected naming/interface diagnostic for non-interface struct, got: %v", diags2)
	}
}

func TestInterfaceEmbeddingAndCollisions(t *testing.T) {
	codeValid := `
type IfcIdentifiable = interface {
    id: int,
}
type IfcNamed = interface {
    name: str,
}
type IfcUser = interface {
    IfcIdentifiable,
    IfcNamed,
    ro created_at: i64,
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(codeValid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error in valid interface embedding: %v", d)
		}
	}

	codeCollision := `
type IfcA = interface {
    id: int,
}
type IfcB = interface {
    id: str,
}
type IfcConflict = interface {
    IfcA,
    IfcB,
}
`
	_, diags2, err := analyzer.AnalyzeSource([]byte(codeCollision))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasCollision := false
	for _, d := range diags2 {
		if d.Rule == "type/interface-embedding-collision" {
			hasCollision = true
		}
	}
	if !hasCollision {
		t.Errorf("expected type/interface-embedding-collision, got: %v", diags2)
	}
}

func TestInterfacePrivateFieldProhibited(t *testing.T) {
	code := `
type IfcSecret = interface {
    _token: str,
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasPrivErr := false
	for _, d := range diags {
		if d.Rule == "type/interface-private-field" {
			hasPrivErr = true
		}
	}
	if !hasPrivErr {
		t.Errorf("expected type/interface-private-field, got: %v", diags)
	}
}

func TestInterfaceDuckTypingAndSatisfaction(t *testing.T) {
	code := `
type IfcPoint = interface {
    x: int,
    ro y: int,
}

type Point2D = struct {
    x: int,
    y: int,
    extra: str,
    _hidden: int,
}

type MissingPoint = struct {
    x: int,
}

type BadMutPoint = struct {
    ro x: int,
    y: int,
}

type EnumPoint = enum {
    Origin,
}

fn inspect_point<T where { T satisfies IfcPoint }>(p: T) int {
    return p.x + p.y
}

fn test_satisfaction() void {
    let p = Point2D::{ .x: 1, .y: 2, .extra: "ok", ._hidden: 99 }
    let s = inspect_point(p)

    let bad1 = MissingPoint::{ .x: 5 }
    let s_bad1 = inspect_point(bad1)

    let bad2 = BadMutPoint::{ .x: 5, .y: 6 }
    let s_bad2 = inspect_point(bad2)

    let bad3 = EnumPoint::Origin
    let s_bad3 = inspect_point(bad3)
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasMissingField, hasMutMismatch, hasEnumRejected bool
	for _, d := range diags {
		if d.Rule == "type/interface-missing-field" {
			hasMissingField = true
		}
		if d.Rule == "type/field-mutability-mismatch" {
			hasMutMismatch = true
		}
		if d.Rule == "type/interface-enum-rejected" {
			hasEnumRejected = true
		}
	}

	if !hasMissingField {
		t.Errorf("expected type/interface-missing-field, got: %v", diags)
	}
	if !hasMutMismatch {
		t.Errorf("expected type/field-mutability-mismatch, got: %v", diags)
	}
	if !hasEnumRejected {
		t.Errorf("expected type/interface-enum-rejected, got: %v", diags)
	}
}

func TestStringInterpolationPrimitives(t *testing.T) {
	code := `
fn main() void {
    let name = "Lone"
    let ver = 6
    let active = true
    let msg = f"Running ${name} v0.${ver}.0 (active: ${active})"
}
`
	analyzer := NewAnalyzer()
	prog, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics, got: %v", diags)
	}

	fnDec := prog.Decls[0].(*TFnDec)
	bMsg := fnDec.Body.Stmts[3].(*TVarDec)
	interp, ok := bMsg.Value.(*TInterpolatedStringExpr)
	if !ok {
		t.Fatalf("expected TInterpolatedStringExpr, got %#v", bMsg.Value)
	}
	if interp.ExprType() != TypeStr {
		t.Errorf("expected TypeStr, got %s", interp.ExprType().String())
	}
}

func TestStringInterpolationMissingTrDisplay(t *testing.T) {
	code := `
type Point = struct {
    x: int,
    y: int,
}

fn main() void {
    let p = Point::{ .x: 10, .y: 20 }
    let s = f"Coord: ${p}"
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasMissingTrait := false
	for _, d := range diags {
		if d.Rule == "type/missing-trait-impl" && strings.Contains(d.Message, "does not implement trait \"TrDisplay\"") {
			hasMissingTrait = true
			break
		}
	}
	if !hasMissingTrait {
		t.Fatalf("expected type/missing-trait-impl diagnostic, got: %v", diags)
	}
}

func TestStringInterpolationWithTrDisplay(t *testing.T) {
	code := `
type Point = struct {
    x: int,
    y: int,
}

impl TrDisplay on Point {
    fn fmt(self, buf: any) void {
    }
}

fn main() void {
    let p = Point::{ .x: 10, .y: 20 }
    let s = f"Coord: ${p}"
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics for struct implementing TrDisplay, got: %v", diags)
	}
}

func TestStructFieldWriteThroughRef(t *testing.T) {
	code := `
type Point = struct {
    ro id: int,
    x: int,
    y: int,
}

fn move_point(p: ref<Point>) void {
    p.x = p.x + 10
    p.y = p.y + 20
}

fn main() void {
    var pt = Point::{ .id: 1, .x: 0, .y: 0 }
    move_point(&pt)
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics for field write through mutable ref, got: %v", diags)
	}
}

func TestStructFieldWriteReadOnlyRejections(t *testing.T) {
	code := `
type Point = struct {
    ro id: int,
    x: int,
}

fn test_ro_ref(r: ref<ro Point>) void {
    r.x = 42
}

fn test_ro_field(r: ref<Point>) void {
    r.id = 99
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasRefWriteErr := false
	hasFieldWriteErr := false
	for _, d := range diags {
		if d.Rule == "type/immutable-reference-write" {
			hasRefWriteErr = true
		}
		if d.Rule == "type/immutable-field-write" {
			hasFieldWriteErr = true
		}
	}

	if !hasRefWriteErr {
		t.Errorf("expected type/immutable-reference-write diagnostic for ref<ro Point>, got: %v", diags)
	}
	if !hasFieldWriteErr {
		t.Errorf("expected type/immutable-field-write diagnostic for ro field through ref, got: %v", diags)
	}
}

func TestStructByValueParameterImmutability(t *testing.T) {
	code := `
type Point = struct {
    x: int,
}

fn mutate_param(p: Point) void {
    p.x = 100
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasReassignErr := false
	for _, d := range diags {
		if d.Rule == "type/immutable-reassignment" {
			hasReassignErr = true
			if !strings.Contains(d.Message, `cannot mutate field "x" on immutable binding "p"`) {
				t.Errorf("unexpected message: %s", d.Message)
			}
			break
		}
	}

	if !hasReassignErr {
		t.Fatalf("expected type/immutable-reassignment diagnostic, got: %v", diags)
	}
}

func TestSinglePointerIndexingRejected(t *testing.T) {
	code := `
fn test_single_ptr(p: ptr<int>) void {
    let val = p[0]
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasSinglePtrErr := false
	for _, d := range diags {
		if d.Rule == "type/single-pointer-indexing-rejected" {
			hasSinglePtrErr = true
			if !strings.Contains(d.Message, `cannot index single-item pointer "ptr<i32>"`) {
				t.Errorf("unexpected message: %s", d.Message)
			}
			break
		}
	}
	if !hasSinglePtrErr {
		t.Fatalf("expected type/single-pointer-indexing-rejected diagnostic, got: %v", diags)
	}
}

func TestSizedPointerConstantBoundsCheck(t *testing.T) {
	code := `
fn test_sized_ptr(p: ptr<[6]int>) void {
    let ok0 = p[0]
    let ok5 = p[5]
    let bad6 = p[6]
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasOobErr := false
	for _, d := range diags {
		if d.Rule == "type/index-out-of-bounds" {
			hasOobErr = true
			if !strings.Contains(d.Message, `index 6 is out of bounds for array pointer "ptr<[6]i32>" of size 6`) {
				t.Errorf("unexpected message: %s", d.Message)
			}
			break
		}
	}
	if !hasOobErr {
		t.Fatalf("expected type/index-out-of-bounds diagnostic, got: %v", diags)
	}
}

func TestUnsizedBufferPointerAllowed(t *testing.T) {
	code := `
fn test_unsized_ptr(buf: ptr<[]int>) int {
    let v0 = buf[0]
    buf[100] = 42
    let deref = *buf
    return v0 + deref
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics for unsized buffer pointer, got: %v", diags)
	}
}

func TestSizedPointerDecayToUnsized(t *testing.T) {
	code := `
fn process_buffer(b: ptr<[]int>) void {}

fn test_decay(p: ptr<[6]int>) void {
    process_buffer(p)
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics for safe decay ptr<[6]int> -> ptr<[]int>, got: %v", diags)
	}
}

func TestRefBufferIndexingAndSingleRefRejection(t *testing.T) {
	code := `
fn test_ref_buffer(r_sized: ref<[4]int>, r_slice: ref<[]int>, r_single: ref<int>) void {
    let a = r_sized[0]
    let b = r_slice[1]
    let bad = r_single[0]
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasInvalidIndexErr := false
	for _, d := range diags {
		if d.Rule == "type/invalid-index" {
			hasInvalidIndexErr = true
			if !strings.Contains(d.Message, `cannot index managed reference of non-buffer type "ref<i32>"`) {
				t.Errorf("unexpected message: %s", d.Message)
			}
			break
		}
	}
	if !hasInvalidIndexErr {
		t.Fatalf("expected type/invalid-index diagnostic for single ref, got: %v", diags)
	}
}

func TestReadOnlyBufferWriteRejection(t *testing.T) {
	code := `
fn test_ro_buf(p_ro: ptr<ro []int>, r_ro: ref<ro []int>) void {
    p_ro[0] = 10
    r_ro[0] = 20
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasPtrWriteErr := false
	hasRefWriteErr := false
	for _, d := range diags {
		if d.Rule == "type/immutable-pointer-write" {
			hasPtrWriteErr = true
		}
		if d.Rule == "type/immutable-reference-write" {
			hasRefWriteErr = true
		}
	}
	if !hasPtrWriteErr {
		t.Errorf("expected type/immutable-pointer-write diagnostic for p_ro[0], got: %v", diags)
	}
	if !hasRefWriteErr {
		t.Errorf("expected type/immutable-reference-write diagnostic for r_ro[0], got: %v", diags)
	}
}



