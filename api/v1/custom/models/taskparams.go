package models

import (
	"time"

	"github.com/soopsio/mgmt/api/v1/models"
)

// 用于 orm
// 新建任务的操作日志
type HistoryTaskParams struct {
	ID               int64       `xorm:"pk autoincr 'id'"`
	TaskID           string      `xorm:"notnull unique 'task_id'"`
	AccessKeyID      string      `xorm:"notnull 'access_key_id'"`
	Signature        string      `xorm:"notnull 'signature'"`
	SignatureMethod  string      `xorm:"notnull 'signature_method'"`
	SignatureNonce   string      `xorm:"notnull 'signature_nonce'"`
	SignatureVersion string      `xorm:"notnull 'signature_version'"`
	Timestamp        string      `xorm:"notnull 'timestamp'"`
	Version          string      `xorm:"notnull 'version'"`
	Task             models.Task `xorm:"json"`
	CreatedAt        time.Time   `xorm:"created"`
	UpdatedAt        time.Time   `xorm:"updated"`
}

func (h *HistoryTaskParams) TableName() string {
	return DEFAULT_TABLE_PREFIX + "history_task_params"
}
