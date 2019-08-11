package jet

import (
	"bytes"
	"github.com/go-jet/jet/internal/utils"
	"github.com/google/uuid"
	"strconv"
	"strings"
	"time"
)

type SqlBuilder struct {
	Dialect Dialect
	Buff    bytes.Buffer
	Args    []interface{}

	lastChar byte
	ident    int
}

func (s *SqlBuilder) DebugSQL() string {
	return queryStringToDebugString(s.Buff.String(), s.Args, s.Dialect)
}

const defaultIdent = 5

func (q *SqlBuilder) IncreaseIdent(ident ...int) {
	if len(ident) > 0 {
		q.ident += ident[0]
	} else {
		q.ident += defaultIdent
	}
}

func (q *SqlBuilder) DecreaseIdent(ident ...int) {
	toDecrease := defaultIdent

	if len(ident) > 0 {
		toDecrease = ident[0]
	}

	if q.ident < toDecrease {
		q.ident = 0
	}

	q.ident -= toDecrease
}

func (q *SqlBuilder) WriteProjections(statement StatementType, projections []Projection) error {
	q.IncreaseIdent()
	err := SerializeProjectionList(statement, projections, q)
	q.DecreaseIdent()
	return err
}

func (q *SqlBuilder) writeFrom(statement StatementType, table Serializer) error {
	q.NewLine()
	q.WriteString("FROM")

	q.IncreaseIdent()
	err := table.serialize(statement, q)
	q.DecreaseIdent()

	return err
}

func (q *SqlBuilder) writeWhere(statement StatementType, where Expression) error {
	q.NewLine()
	q.WriteString("WHERE")

	q.IncreaseIdent()
	err := where.serialize(statement, q, noWrap)
	q.DecreaseIdent()

	return err
}

func (q *SqlBuilder) writeGroupBy(statement StatementType, groupBy []GroupByClause) error {
	q.NewLine()
	q.WriteString("GROUP BY")

	q.IncreaseIdent()
	err := serializeGroupByClauseList(statement, groupBy, q)
	q.DecreaseIdent()

	return err
}

func (q *SqlBuilder) writeOrderBy(statement StatementType, orderBy []OrderByClause) error {
	q.NewLine()
	q.WriteString("ORDER BY")

	q.IncreaseIdent()
	err := serializeOrderByClauseList(statement, orderBy, q)
	q.DecreaseIdent()

	return err
}

func (q *SqlBuilder) writeHaving(statement StatementType, having Expression) error {
	q.NewLine()
	q.WriteString("HAVING")

	q.IncreaseIdent()
	err := having.serialize(statement, q, noWrap)
	q.DecreaseIdent()

	return err
}

func (q *SqlBuilder) WriteReturning(statement StatementType, returning []Projection) error {
	if len(returning) == 0 {
		return nil
	}

	q.NewLine()
	q.WriteString("RETURNING")
	q.IncreaseIdent()

	return q.WriteProjections(statement, returning)
}

func (q *SqlBuilder) NewLine() {
	q.write([]byte{'\n'})
	q.write(bytes.Repeat([]byte{' '}, q.ident))
}

func (q *SqlBuilder) write(data []byte) {
	if len(data) == 0 {
		return
	}

	if !isPreSeparator(q.lastChar) && !isPostSeparator(data[0]) && q.Buff.Len() > 0 {
		q.Buff.WriteByte(' ')
	}

	q.Buff.Write(data)
	q.lastChar = data[len(data)-1]
}

func isPreSeparator(b byte) bool {
	return b == ' ' || b == '.' || b == ',' || b == '(' || b == '\n' || b == ':'
}

func isPostSeparator(b byte) bool {
	return b == ' ' || b == '.' || b == ',' || b == ')' || b == '\n' || b == ':'
}

func (q *SqlBuilder) writeAlias(str string) {
	aliasQuoteChar := string(q.Dialect.AliasQuoteChar())
	q.WriteString(aliasQuoteChar + str + aliasQuoteChar)
}

func (q *SqlBuilder) WriteString(str string) {
	q.write([]byte(str))
}

func (q *SqlBuilder) writeIdentifier(name string, alwaysQuote ...bool) {
	quoteWrap := name != strings.ToLower(name) || strings.ContainsAny(name, ". -")

	if quoteWrap || len(alwaysQuote) > 0 {
		identQuoteChar := string(q.Dialect.IdentifierQuoteChar())
		q.WriteString(identQuoteChar + name + identQuoteChar)
	} else {
		q.WriteString(name)
	}
}

func (q *SqlBuilder) writeByte(b byte) {
	q.write([]byte{b})
}

func (q *SqlBuilder) finalize() (string, []interface{}) {
	return q.Buff.String() + ";\n", q.Args
}

func (q *SqlBuilder) insertConstantArgument(arg interface{}) {
	q.WriteString(argToString(arg))
}

func (q *SqlBuilder) insertParametrizedArgument(arg interface{}) {
	q.Args = append(q.Args, arg)
	argPlaceholder := q.Dialect.ArgumentPlaceholder()(len(q.Args))

	q.WriteString(argPlaceholder)
}

func argToString(value interface{}) string {
	if utils.IsNil(value) {
		return "NULL"
	}

	switch bindVal := value.(type) {
	case bool:
		if bindVal {
			return "TRUE"
		}
		return "FALSE"
	case int8:
		return strconv.FormatInt(int64(bindVal), 10)
	case int:
		return strconv.FormatInt(int64(bindVal), 10)
	case int16:
		return strconv.FormatInt(int64(bindVal), 10)
	case int32:
		return strconv.FormatInt(int64(bindVal), 10)
	case int64:
		return strconv.FormatInt(int64(bindVal), 10)

	case uint8:
		return strconv.FormatUint(uint64(bindVal), 10)
	case uint:
		return strconv.FormatUint(uint64(bindVal), 10)
	case uint16:
		return strconv.FormatUint(uint64(bindVal), 10)
	case uint32:
		return strconv.FormatUint(uint64(bindVal), 10)
	case uint64:
		return strconv.FormatUint(uint64(bindVal), 10)

	case float32:
		return strconv.FormatFloat(float64(bindVal), 'f', -1, 64)
	case float64:
		return strconv.FormatFloat(float64(bindVal), 'f', -1, 64)

	case string:
		return stringQuote(bindVal)
	case []byte:
		return stringQuote(string(bindVal))
	case uuid.UUID:
		return stringQuote(bindVal.String())
	case time.Time:
		return stringQuote(string(utils.FormatTimestamp(bindVal)))
	default:
		return "[Unsupported type]"
	}
}

func stringQuote(value string) string {
	return `'` + strings.Replace(value, "'", "''", -1) + `'`
}
