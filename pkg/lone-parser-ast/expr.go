package loneast

type ExprKind int

const (
	EXPR_LITERAL ExprKind = iota
	EXPR_OBJ_LITERAL
	EXPR_LIST_LITERAL
	EXPR_TRAIT_MATERIALIZATION
	EXPR_NAME_REFERENCE
	EXPR_UNARY_OP
	EXPR_BINARY_OP
	EXPR_LABEL_DEC
	EXPR_BREAK
	EXPR_IF
	EXPR_WHILE
	EXPR_FOR
	EXPR_DROP
)

type Expr interface {
	IsExpr() ExprKind
}

// =====================
// BINARY EXPR
// =====================

type BinaryOp string

const (
	BINARY_OP_PLUS       BinaryOp = "+"
	BINARY_OP_MINUS      BinaryOp = "-"
	BINARY_OP_TIME       BinaryOp = "*"
	BINARY_OP_DIV        BinaryOp = "/"
	BINARY_OP_MOD        BinaryOp = "%"
	BINARY_OP_AND        BinaryOp = "and"
	BINARY_OP_OR         BinaryOp = "or"
	BINARY_OP_XOR        BinaryOp = "xor"
	BINARY_OP_CARET      BinaryOp = "^"
	BINARY_OP_LESS       BinaryOp = ">"
	BINARY_OP_LESS_EQ    BinaryOp = ">="
	BINARY_OP_EQ         BinaryOp = "=="
	BINARY_OP_NOT_EQ     BinaryOp = "!="
	BINARY_OP_GREATER    BinaryOp = "<"
	BINARY_OP_GREATER_EQ BinaryOp = "<="
)

type BinaryExpr struct {
	Op    BinaryOp
	Left  Expr
	Right Expr
	Pos   Position
}

func (BinaryExpr) IsExpr() ExprKind {
	return EXPR_BINARY_OP
}

// =====================
// UNARY EXPR
// =====================

type UnaryOp string

const (
	UNARY_OP_POSITIVE = "+"
	UNARY_OP_NEGATIVE = "-"
	UNARY_OP_NOT      = "!"
	UNARY_OP_RAW_REF  = "&raw"
	UNARY_OP_REF      = "&"
	UNARY_OP_DEREF    = "*"
)

type UnaryExpr struct {
	Op   UnaryOp
	Expr Expr
	Pos  Position
}

func (UnaryExpr) IsExpr() ExprKind {
	return EXPR_UNARY_OP
}

// =====================
// POSTFIX EXPR
// =====================

type SuffixKind int

const (
	SUFFIX_KIND_MEMBER = iota
	SUFFIX_KIND_ASSOCIATED
	SUFFIX_KIND_NAMESPACE
	SUFFIX_KIND_CALL
	SUFFIX_KIND_CALL_TYPE_INIT
	SUFFIX_KIND_CALL_TYPE_MAKE
	SUFFIX_KIND_INDEX
	SUFFIX_KIND_TRAIT_MAT
)

type Suffix interface {
	IsSuffix() SuffixKind
}

type PostFixExpr struct {
	Base   Expr
	Suffix []Suffix
	Pos    Position
}

func (PostFixExpr) IsExpr() ExprKind {
	return EXPR_NAME_REFERENCE
}

type SuffixCall struct {
	Args []Expr
	Pos  Position
}

func (SuffixCall) IsSuffix() SuffixKind {
	return SUFFIX_KIND_CALL
}

type SuffixCallTypeInit struct {
	Args []Expr
	Pos  Position
}

func (SuffixCallTypeInit) IsSuffix() SuffixKind {
	return SUFFIX_KIND_CALL_TYPE_INIT
}

type TypeMakeKind = int

const (
	CALL_TYPE_MAKE_OBJ TypeMakeKind = iota
	CALL_TYPE_MAKE_LIST
)

type SuffixCallTypeMake struct {
	Kind TypeMakeKind
	Arg  any
	Pos  Position
}

func (SuffixCallTypeMake) IsSuffix() SuffixKind {
	return SUFFIX_KIND_CALL_TYPE_MAKE
}

type SuffixMemberAccess struct {
	Name string
	Pos  Position
}

func (SuffixMemberAccess) IsSuffix() SuffixKind {
	return SUFFIX_KIND_MEMBER
}

type SuffixAssociatedAccess struct {
	Name string
	Pos  Position
}

func (SuffixAssociatedAccess) IsSuffix() SuffixKind {
	return SUFFIX_KIND_ASSOCIATED
}

type SuffixNamespaceAccess struct {
	Name string
	Pos  Position
}

func (SuffixNamespaceAccess) IsSuffix() SuffixKind {
	return SUFFIX_KIND_NAMESPACE
}

type SuffixIndex struct {
	Index Expr
	Pos   Position
}

func (SuffixIndex) IsSuffix() SuffixKind {
	return SUFFIX_KIND_INDEX
}

type SuffixTraitMat struct {
	TargetType TYPE_REF
	Pos        Position
}

func (SuffixTraitMat) IsSuffix() SuffixKind {
	return SUFFIX_KIND_TRAIT_MAT
}

// =====================
// LITERAL EXPR
// =====================

type LiteralKind int

const (
	LITERAL_BOOL = iota
	LITERAL_NONE
	LITERAL_STRING
	LITERAL_INT
	LITERAL_FLOAT
	LITERAL_UNINITIALIZED
	LITERAL_DYNAMIC_SIZE
)

type Literal struct {
	Kind  LiteralKind
	Value any
	Pos   Position
}

func (Literal) IsExpr() ExprKind {
	return EXPR_LITERAL
}

// =====================
// NAME REF EXPR
// =====================

type NameRef struct {
	Name string
	Pos  Position
}

func (NameRef) IsExpr() ExprKind {
	return EXPR_NAME_REFERENCE
}

// =====================
// OBJ LITERAL EXPR
// =====================

type ObjectLiteral struct {
	Fields []FieldAssign `json:"fields"`
	Pos    Position
}

func (ObjectLiteral) IsExpr() ExprKind {
	return EXPR_OBJ_LITERAL
}

type FieldAssign struct {
	Name string `json:"name"`
	Expr Expr   `json:"expr"`
}

// =====================
// LIST LITERAL EXPR
// =====================

type ListLiteral struct {
	Content []Expr
	Pos     Position
}

func (ListLiteral) IsExpr() ExprKind {
	return EXPR_LIST_LITERAL
}

// =====================
// LABEL DEC EXPR
// =====================

type LabelDec struct {
	Name string
	Body Block
	Pos  Position
}

func (LabelDec) IsExpr() ExprKind {
	return EXPR_LABEL_DEC
}

// =====================
// BREAK STMT / EXPR
// =====================

type BreakStmt struct {
	Label string
	Value Expr
	Pos   Position
}

func (BreakStmt) IsExpr() ExprKind {
	return EXPR_BREAK
}

// =====================
// IF STMT / EXPR
// =====================

type IfStmt struct {
	Cond Expr
	Then Block
	Else any // nil, Block, or IfStmt
	Pos  Position
}

func (IfStmt) IsExpr() ExprKind {
	return EXPR_IF
}

// =====================
// WHILE STMT / EXPR
// =====================

type WhileStmt struct {
	Cond Expr
	Body Block
	Pos  Position
}

func (WhileStmt) IsExpr() ExprKind {
	return EXPR_WHILE
}

// =====================
// FOR STMT / EXPR
// =====================

type ForStmt struct {
	VarName string
	Iter    Expr
	Body    Block
	Pos     Position
}

func (ForStmt) IsExpr() ExprKind {
	return EXPR_FOR
}

// =====================
// DROP STMT / EXPR
// =====================

type DropStmt struct {
	Expr Expr
	Pos  Position
}

func (DropStmt) IsExpr() ExprKind {
	return EXPR_DROP
}
