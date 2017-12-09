package main

import (
	"encoding/json"
	"flag"
	"log"
	"runtime"

	"github.com/astaxie/beego"
	beconfig "github.com/astaxie/beego/config"

	"github.com/soopsio/mgmt/routers"
	"go.uber.org/config"
	"go.uber.org/zap"

	"github.com/go-xorm/xorm"
	"github.com/jmoiron/sqlx"
	"github.com/soopsio/mgmt/api/v1/custom/ladon"
	apimodels "github.com/soopsio/mgmt/api/v1/custom/models"
	"github.com/soopsio/mgmt/api/v1/custom/processer"
	"github.com/soopsio/mgmt/api/v1/restapi"
	mgmtconfig "github.com/soopsio/mgmt/config"

	"github.com/soopsio/zlog"
	"github.com/soopsio/zlog/zlogbeat/cmd"
)

func checkError(err error) {
	if err != nil {
		// debug.PrintStack()
		logger.Error("Error:" + err.Error())
	}
}

var (
	args    []string
	out     []byte
	err     error
	cfg     beconfig.Configer
	logger  *zap.Logger
	orm     *xorm.Engine
	db      *sqlx.DB
	cfgfile = flag.String("logconf", "conf/log.yml", "main log config file.")
)

func initLogger() {
	cmd.RootCmd.Flags().AddGoFlag(flag.CommandLine.Lookup("logconf"))
	p, err := config.NewYAMLProviderFromFiles(*cfgfile)
	if err != nil {
		log.Fatalln(err)
	}

	sw := zlog.NewWriteSyncer(p)
	conf := zap.NewProductionConfig()
	conf.DisableCaller = true
	conf.Encoding = "json"

	logger, _ = conf.Build(zlog.SetOutput(sw, conf))
}

func init() {
	initLogger()

	if err := GlobalInit(); err != nil {
		log.Fatalln("初始化失败", err)
	}
	logger.Info("初始化完成")
}

func GlobalInit() error {
	// logger, err = zap.NewProduction()
	log.Println("logger 初始地址：：", logger)
	// 获取配置实例
	cfg = mgmtconfig.GetConfig()

	// 初始化orm
	logger.Info("初始化 orm")
	if err := apimodels.Init(cfg); err != nil {
		return err
	}

	orm = apimodels.GetXormEngine()
	db = apimodels.GetSqlxDB()

	// 初始化api
	logger.Info("初始化 API")
	apiconfig := restapi.NewApiConfig()
	apiconfig.SetLogger(logger)

	// 初始化路由
	logger.Info("初始化路由")
	routers.Init(orm, logger)

	// 初始化任务处理器
	logger.Info("任务处理器")
	processer.NewProcesserConfig().SetLogger(logger)

	// 初始化ladon
	logger.Info("初始化 ladon")
	ladon_ins := ladon.New()
	ladon_ins.SetLogger(logger)
	ladon_ins.SetDB(db)
	return nil
}

// mainController 控制器入口
type mainController struct {
	beego.Controller
}

func (m *mainController) Get() {
	// m.TplName = "index.html"
	m.Data["json"] = "hello"
	m.ServeJSON()
}

type stateController struct {
	beego.Controller
}

type memStats struct {
	Alloc      uint64 `json:"alloc"`
	TotalAlloc uint64 `json:"totalalloc"`
	HeapAlloc  uint64 `json:"heapalloc"`
	HeapSys    uint64 `json:"heapsys"`
}

func (s *stateController) Get() {
	var mema runtime.MemStats
	runtime.ReadMemStats(&mema)
	memb := memStats{mema.Alloc, mema.TotalAlloc, mema.HeapAlloc, mema.HeapSys}
	memStats, _ := json.Marshal(memb)
	s.Data["json"] = string(memStats)
	s.ServeJSON()
}
func main() {
	flag.Parse()
	beego.Router("/", &mainController{})
	beego.Router("/status", &stateController{})
	beego.Run()
}
