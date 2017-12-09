/**
鉴权
*/
package ladon

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/ory/ladon"
	sqlmanager "github.com/ory/ladon/manager/sql"
	"go.uber.org/zap"
)

var (
	lc *ladonConfig
)

func init() {
	lc = &ladonConfig{}
}

type ladonConfig struct {
	l  *zap.Logger
	db *sqlx.DB
}

func (la *Ladon) SetLogger(l *zap.Logger) error {
	if l != nil {
		la.cfg.l = l
	}
	return nil
}

func (la *Ladon) SetDB(db *sqlx.DB) error {
	if db != nil {
		la.cfg.db = db
	}
	return nil
}

type Ladon struct {
	ladon.Ladon
	manager *sqlmanager.SQLManager
	cfg     *ladonConfig
}

func New() *Ladon {
	logger, _ := zap.NewProduction()
	return &Ladon{
		cfg: &ladonConfig{l: logger},
	}
}

// 初始化
func (la *Ladon) Setup() error {
	ladon.ConditionFactories[new(IPFilterCondition).GetName()] = func() ladon.Condition {
		return new(IPFilterCondition)
	}

	la.manager = sqlmanager.NewSQLManager(la.cfg.db, nil)
	return nil
}

func (la *Ladon) IsAllowed(subject, action, resource string, context map[string]interface{}) error {

	// 拼装请求
	req := &ladon.Request{
		Subject:  subject,
		Action:   action,
		Resource: resource,
		Context:  context,
	}
	// 验证权限
	return la.Ladon.IsAllowed(req)
}

// 初始化
func NewPolicy() Policy {
	return Policy{}
}

// 新增策略
func (la *Ladon) CreatePolicy(pol ladon.Policy) error {
	// var pol = &ladon.DefaultPolicy{
	// 	ID:          "68819e5a-738b-41ec-b03c-b58a1b19d043",
	// 	Description: "something humanly readable",
	// 	Subjects:    []string{"max", "peter", "<zac|ken>"},
	// 	Resources:   []string{"myrn:some.domain.com:resource:123", "myrn:some.domain.com:resource:345", "myrn:something:foo:<.+>"},
	// 	Actions:     []string{"<create|delete>", "get"},
	// 	Effect:      ladon.AllowAccess,
	// 	Conditions: ladon.Conditions{
	// 		"owner": &ladon.EqualsSubjectCondition{}, //必须等于 Subjects
	// 		"custom": &IPFilterCondition{
	// 			CIDRs: "127.0.0.1/32,127.0.0.1/32,10.0.0.41",
	// 		},
	// 	},
	// }

	if err := la.manager.Create(pol); err != nil {
		return err
		// if strings.HasPrefix(err.Error(), "Error 1146") {
		// 	if err := li.manager.CreateSchemas(); err != nil {
		// 		return err
		// 	}
		// 	if err := li.Manager.Create(pol); err != nil {
		// 		return err
		// 	}
		// } else {
		// 	return err
		// }
	}
	return nil
}

// 根据主题（用户名）查找匹配策略
func (la *Ladon) FindPoliciesForSubject(subject string) (ladon.Policies, error) {
	ps := ladon.Policies{}
	var limit, offset int64 = 1, 0
	for {
		pols, err := la.manager.GetAll(limit, offset)
		if err != nil {
			return ps, err
		}
		for _, p := range pols {
			for _, s := range p.GetSubjects() {
				if s == subject {
					ps = append(ps, p)
				}
			}
		}
		offset += limit
	}
	return ps, nil

}

// 根据策略ID 查找
func (la *Ladon) GetByID(id string) (ladon.Policy, error) {
	return la.manager.Get(id)
}

// 删除指定策略
func (la *Ladon) DeleteByID(id string) error {
	return la.manager.Delete(id)
}
