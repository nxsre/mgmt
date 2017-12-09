package processer

import (
	"log"
	"strconv"
	"sync"

	machinery "github.com/RichardKnop/machinery/v1"
	"github.com/go-redis/redis"
	"github.com/soopsio/mgmt/models"
	"go.uber.org/zap"

	machineryconfig "github.com/RichardKnop/machinery/v1/config"
	mgmtconfig "github.com/soopsio/mgmt/config"
)

var lock = &sync.Mutex{}
var cfg = mgmtconfig.GetConfig()
var server *machinery.Server
var cnf = &machineryconfig.Config{
	Broker:        "redis://" + cfg.DefaultString("machinery::broker", "127.0.0.1:6379") + "/" + strconv.Itoa(cfg.DefaultInt("machinery::broker_db", 0)),
	DefaultQueue:  cfg.DefaultString("machinery::queue", "machinery_task"),
	ResultBackend: "redis://" + cfg.DefaultString("machinery::result_backend", "127.0.0.1:6379") + "/" + strconv.Itoa(cfg.DefaultInt("machinery::result_backend_db", 0)),
}

var logger *zap.Logger
var Redis *redis.Client
var err error

func init() {
	logger, _ = zap.NewProduction()
	Redis = models.NewRedisClient()

	server, err = machinery.NewServer(cnf)
	if err != nil {
		// do something with the error
	}
	// Register tasks
	ts := map[string]interface{}{
		"ansible": Ansible,
	}

	server.RegisterTasks(ts)

	worker := server.NewWorker("ansible_mgmt_001", cfg.DefaultInt("machinery::concurrency", 500))
	go func() {
		err := worker.Launch()
		log.Fatalln(err)
	}()
}

type ProcesserConfig struct {
	l *zap.Logger
}

func (pc *ProcesserConfig) SetLogger(l *zap.Logger) error {
	if l != nil {
		pc.l = l
	}
	logger = pc.l
	return nil
}

// 初始化 API 配置
func NewProcesserConfig() *ProcesserConfig {
	log, _ := zap.NewProduction()
	logger = log
	return &ProcesserConfig{
		l: log,
	}
}
