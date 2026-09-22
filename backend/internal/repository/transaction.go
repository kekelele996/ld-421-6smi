package repository

import (
	"context"

	"gorm.io/gorm"
)

type txContextKey struct{}

// withTx 将事务句柄放入 context，供仓储方法复用。
func withTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// dbFromContext 优先使用 context 中的事务，否则退回默认数据库句柄。
func dbFromContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}

// RunInTransaction 在默认数据库上执行事务，事务内仓储调用共享同一事务句柄。
func RunInTransaction(ctx context.Context, db *gorm.DB, fn func(txCtx context.Context) error) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(withTx(ctx, tx))
	})
}
