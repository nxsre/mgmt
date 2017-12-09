package models

import (
	"github.com/astaxie/beego/config"
	"github.com/go-xorm/xorm"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

const (
	DEFAULT_TABLE_PREFIX = "ansible_"
)

var (
	logger   *zap.Logger
	orm      *xorm.Engine
	sqldb    *sqlx.DB
	myconfig map[string]string
)

func SetLogger(l *zap.Logger) error {
	if l != nil {
		logger = l
	}
	return nil
}

func Init(cfg config.Configer) error {
	if mc, err := cfg.GetSection("mysql"); err != nil {
		return err
	} else {
		myconfig = mc
	}

	if err := setXormEngine(); err != nil {
		return err
	}

	if err := setSqlxDB(); err != nil {
		return err
	}

	// 同步表结构
	err := orm.Sync2(new(HistoryTaskParams), new(TaskState), new(AccessKey))
	if err != nil {
		logger.Error(err.Error())
	} else {
		if c, err := orm.Count(&AccessKey{}); err == nil {
			if c == 0 {
				adminUser := &AccessKey{
					KeyID:     GenAccessKeyId(),
					KeySecret: GenAccessKeySecret(),
					IsAdmin:   1,
				}
				if _, err := orm.InsertOne(adminUser); err != nil {
					return err
				} else {
					logger.Info("初始化用户成功", zap.String("adminKeyID", adminUser.KeyID))
				}
			}
		}
	}
	return nil
}
