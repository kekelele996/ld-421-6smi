package service

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// isDuplicateKeyErr 判断是否为唯一索引冲突（生产 MySQL 与测试 SQLite 均覆盖）。
func isDuplicateKeyErr(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
