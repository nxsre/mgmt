package models

import (
	"time"

	"github.com/soopsio/go-utils/rand"
)

// 用户列表
type AccessKey struct {
	ID        int64     `xorm:"pk autoincr 'id'"`
	KeyID     string    `xorm:"notnull unique 'access_key_id'"`
	KeySecret string    `xorm:"notnull 'access_key_secret'"`
	IsAdmin   int64     `xorm:"int(1) notnull default 0"`
	Comment   string    `xorm:"notnull default ''"`
	NickName  string    `xorm:"notnull default ''"`
	CreatedAt time.Time `xorm:"created"`
	UpdatedAt time.Time `xorm:"updated"`
}

func (a *AccessKey) TableName() string {
	return DEFAULT_TABLE_PREFIX + "access_key"
}

// GetKeyList 返回包含 用户名、用户ID、NickName、Comment 的列表
func GetKeyList() (accesskeys []AccessKey, err error) {
	accesskey := new(AccessKey)
	rows, err := orm.Cols("id,access_key_id,nick_name,comment").Rows(accesskey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(accesskey); err != nil {
			return nil, err
		}
		accesskeys = append(accesskeys, *accesskey)
	}
	return accesskeys, nil
}

// 判断是否是管理员
func IsAdmin(accesskeyid string) (bool, error) {
	accesskey := new(AccessKey)
	if n, err := orm.Where("is_admin = 1 and access_key_id = ?", accesskeyid).Count(accesskey); err != nil {
		return false, err
	} else if n > 0 {
		return true, nil
	}
	return false, nil
}

// 创建用户
func CreateAccessKey(accesskey *AccessKey) error {
	_, err := orm.InsertOne(accesskey)
	return err
}

// 生成账号
func GenAccessKeyId() string {
	return rand.RandomAlphabetic(20)
}

// 生成密码
func GenAccessKeySecret() string {
	return rand.RandomAlphabetic(32)
}
