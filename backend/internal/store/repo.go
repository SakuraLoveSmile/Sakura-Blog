package store

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Entity 泛型仓储约束：实体提供表名、全列清单、写入值与行扫描。
// 约定：Columns() 首列必须是 id；Values() 与 Columns()[1:] 一一对应（不含 id）。
type Entity interface {
	TableName() string
	Columns() []string
	Values() []any
	SetID(id int64)
	ScanRow(row rowScanner) error
}

// QueryOption 查询选项（List/Count 用，可组合）。
type QueryOption func(*querySpec)

// querySpec List/Count 的查询规格。
type querySpec struct {
	where   string
	args    []any
	orderBy string
	limit   int
	offset  int
}

// Where 追加 WHERE 条件（不含 WHERE 关键字），多次调用以 AND 连接。
func Where(cond string, args ...any) QueryOption {
	return func(q *querySpec) {
		if q.where == "" {
			q.where = cond
		} else {
			q.where = "(" + q.where + ") AND (" + cond + ")"
		}
		q.args = append(q.args, args...)
	}
}

// OrderBy 设置 ORDER BY 子句（不含关键字）。
func OrderBy(order string) QueryOption {
	return func(q *querySpec) { q.orderBy = order }
}

// Limit 限制返回行数（0 = 不限制）。
func Limit(n int) QueryOption {
	return func(q *querySpec) { q.limit = n }
}

// Offset 跳过前 n 行（需配合 Limit 使用）。
func Offset(n int) QueryOption {
	return func(q *querySpec) { q.offset = n }
}

// Repository 泛型数据仓储：按 id/条件的单条查询、插入、全列更新、删除与带选项的列表/计数。
// 固定语句（get_by_id/create/update/delete）预编译缓存，List/GetOne 因条件组合多不缓存。
// T 实例化为实体指针类型（如 *Link）；新实例用反射按 typ 构造（new(T) 生成的 *T 不继承方法集）。
type Repository[T Entity] struct {
	db    *sql.DB
	table string
	cols  []string
	typ   reflect.Type // 实体指针类型（如 *Link），反射构造新实例用

	mu    sync.Mutex
	stmts map[string]*sql.Stmt
}

// NewRepository 构造仓储实例。zero 仅用于读取表元数据（TableName/Columns）。
func NewRepository[T Entity](db *sql.DB, zero T) *Repository[T] {
	return &Repository[T]{
		db:    db,
		table: zero.TableName(),
		cols:  zero.Columns(),
		typ:   reflect.TypeOf(zero),
		stmts: map[string]*sql.Stmt{},
	}
}

// newEntity 反射创建实体新实例（如 *Link），按 Entity 接口使用。
func (r *Repository[T]) newEntity() Entity {
	return reflect.New(r.typ.Elem()).Interface().(Entity)
}

// stmt 取或建预编译语句（惰性缓存，幂等）。
func (r *Repository[T]) stmt(key, query string) (*sql.Stmt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if st, ok := r.stmts[key]; ok {
		return st, nil
	}
	st, err := r.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("prepare %s %s: %w", r.table, key, err)
	}
	r.stmts[key] = st
	return st, nil
}

// GetByID 按 id 查单条。未找到返回 nil（T 的零值）。
func (r *Repository[T]) GetByID(id int64) (T, error) {
	st, err := r.stmt("get_by_id", "SELECT "+strings.Join(r.cols, ", ")+" FROM "+r.table+" WHERE id = ?")
	if err != nil {
		var zero T
		return zero, err
	}
	e := r.newEntity()
	if err := e.ScanRow(st.QueryRow(id)); err == sql.ErrNoRows {
		var zero T
		return zero, nil
	} else if err != nil {
		var zero T
		return zero, fmt.Errorf("get %s %d: %w", r.table, id, err)
	}
	return e.(T), nil
}

// GetOne 按条件查单条。未找到返回 nil（T 的零值）。
func (r *Repository[T]) GetOne(where string, args ...any) (T, error) {
	var zero T
	e := r.newEntity()
	err := e.ScanRow(r.db.QueryRow("SELECT "+strings.Join(r.cols, ", ")+" FROM "+r.table+" WHERE "+where+" LIMIT 1", args...))
	if err == sql.ErrNoRows {
		return zero, nil
	}
	if err != nil {
		return zero, fmt.Errorf("get %s: %w", r.table, err)
	}
	return e.(T), nil
}

// Create 插入一条记录（除 id 外全列写入），回填 ID 并返回。
func (r *Repository[T]) Create(e T) (int64, error) {
	insCols := r.cols[1:]
	ph := strings.TrimSuffix(strings.Repeat("?,", len(insCols)), ",")
	st, err := r.stmt("create", "INSERT INTO "+r.table+" ("+strings.Join(insCols, ", ")+") VALUES ("+ph+")")
	if err != nil {
		return 0, err
	}
	res, err := st.Exec(e.Values()...)
	if err != nil {
		return 0, fmt.Errorf("create %s: %w", r.table, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create %s: last insert id: %w", r.table, err)
	}
	e.SetID(id)
	return id, nil
}

// Update 按 id 全列更新（除 id 外）。0 行受影响返回错误。
func (r *Repository[T]) Update(id int64, e T) error {
	sets := make([]string, 0, len(r.cols)-1)
	for _, c := range r.cols[1:] {
		sets = append(sets, c+" = ?")
	}
	st, err := r.stmt("update", "UPDATE "+r.table+" SET "+strings.Join(sets, ", ")+" WHERE id = ?")
	if err != nil {
		return err
	}
	res, err := st.Exec(append(e.Values(), id)...)
	if err != nil {
		return fmt.Errorf("update %s %d: %w", r.table, id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update %s %d: no rows affected", r.table, id)
	}
	return nil
}

// Delete 按 id 删除记录。0 行受影响不报错（幂等语义）。
func (r *Repository[T]) Delete(id int64) error {
	st, err := r.stmt("delete", "DELETE FROM "+r.table+" WHERE id = ?")
	if err != nil {
		return err
	}
	if _, err := st.Exec(id); err != nil {
		return fmt.Errorf("delete %s %d: %w", r.table, id, err)
	}
	return nil
}

// List 按选项查询多条（无匹配返回空切片）。
func (r *Repository[T]) List(opts ...QueryOption) ([]T, error) {
	q := &querySpec{}
	for _, o := range opts {
		o(q)
	}
	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(r.cols, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(r.table)
	if q.where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(q.where)
	}
	if q.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(q.orderBy)
	}
	if q.limit > 0 {
		sb.WriteString(" LIMIT ?")
		q.args = append(q.args, q.limit)
	}
	if q.offset > 0 {
		// SQLite 要求 OFFSET 必须配 LIMIT；无 limit 时用 LIMIT -1 表示不限制
		if q.limit <= 0 {
			sb.WriteString(" LIMIT -1")
		}
		sb.WriteString(" OFFSET ?")
		q.args = append(q.args, q.offset)
	}
	rows, err := r.db.Query(sb.String(), q.args...)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", r.table, err)
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		e := r.newEntity()
		if err := e.ScanRow(rows); err != nil {
			return nil, fmt.Errorf("scan %s: %w", r.table, err)
		}
		out = append(out, e.(T))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", r.table, err)
	}
	return out, nil
}

// Count 按选项计数。
func (r *Repository[T]) Count(opts ...QueryOption) (int, error) {
	q := &querySpec{}
	for _, o := range opts {
		o(q)
	}
	query := "SELECT COUNT(*) FROM " + r.table
	if q.where != "" {
		query += " WHERE " + q.where
	}
	var n int
	if err := r.db.QueryRow(query, q.args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count %s: %w", r.table, err)
	}
	return n, nil
}
