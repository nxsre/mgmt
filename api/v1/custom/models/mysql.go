package models

import (
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"github.com/jmoiron/sqlx"
)

func getMySQLDSN() (string, error) {
	var dsn string

	dsn = myconfig["username"] + ":" + myconfig["password"] + "@tcp(" + myconfig["host"]
	if v, ok := myconfig["port"]; ok {
		dsn += ":" + v
	}

	dsn += ")"

	if v, ok := myconfig["database"]; ok {
		dsn += "/" + v
	}

	dsnparams := []string{}
	for k, v := range myconfig {
		switch k {
		default:
			if v != "" {
				dsnparams = append(dsnparams, k+"="+v)
			}
		case "host", "port", "username", "password", "database":
			// fmt.Println(k, v)
		}
	}
	dsn += "?" + strings.Join(append(dsnparams, "parseTime=true&charset=utf8mb4&collation=utf8_general_ci"), "&")
	return dsn, nil
}

func setXormEngine() error {
	dsn, err := getMySQLDSN()
	if err != nil {
		return err
	}

	if o, err := xorm.NewEngine("mysql", dsn); err != nil {
		return err
	} else {
		orm = o
	}

	orm.ShowSQL()
	orm.ShowExecTime()
	xlog := orm.Logger()
	xlog.SetLevel(0)
	orm.SetLogger(xlog)

	return nil
}

func setSqlxDB() error {
	dsn, err := getMySQLDSN()
	if err != nil {
		return err
	}
	if x, err := sqlx.Connect("mysql", dsn); err != nil {
		return err
	} else {
		sqldb = x
	}
	return nil
}

func GetXormEngine() *xorm.Engine {
	return orm
}

func GetSqlxDB() *sqlx.DB {
	return sqldb
}
