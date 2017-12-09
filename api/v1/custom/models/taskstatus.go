package models

import (
	"time"

	"github.com/soopsio/mgmt/api/v1/models"
)

// 任务状态
type TaskState struct {
	ID          int64  `xorm:"pk autoincr 'id'"`
	TaskID      string `xorm:"notnull unique 'task_id'"`
	models.Task `xorm:"extends"`
	CreatedAt   time.Time `xorm:"created"`
	UpdatedAt   time.Time `xorm:"updated"`
}

func (t *TaskState) TableName() string {
	return DEFAULT_TABLE_PREFIX + "task_state"
}
