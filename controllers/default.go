package controllers

import (
	"encoding/json"
	"log"
	"time"

	"github.com/astaxie/beego"
	"github.com/go-redis/redis"
	"github.com/soopsio/mgmt/api/v1/custom/pipe/message"
	"github.com/soopsio/mgmt/models"
)

var Redis *redis.Client
var err error

func init() {
	Redis = models.NewRedisClient()
}

type MainController struct {
	beego.Controller
}

func (c *MainController) Get() {
	c.Data["Website"] = "beego.me"
	c.Data["Email"] = "astaxie@gmail.com"
	c.TplName = "index.tpl"
}

type TaskIDController struct {
	beego.Controller
}

// 反序列化redis 中存储的 taskid
type Taskid struct {
	Ansible_Task_ID string `json:"ansible_task_id,omitempty"`
	Timeout         string `json:"timeout,omitempty"`
	ErrMsg          string `json:"msg,omitempty"`
}

func (this *TaskIDController) Get() {
	tid := &Taskid{}

	timeout, _ := this.GetInt("timeout", 10)
	strs := Redis.BLPop(time.Duration(timeout)*time.Second, "ansible_task_id_channel_longpoll")
	if len(strs.Val()) > 1 {
		if err := json.Unmarshal([]byte(strs.Val()[1]), tid); err != nil {
			tid.ErrMsg = err.Error()
		}
	} else {
		tid.Timeout = `no events before timeout`
	}
	this.Data["json"] = tid
	this.ServeJSON(true)
}

type TaskEventController struct {
	beego.Controller
}

func (this *TaskEventController) Get() {
	taskID := this.GetString("task_id")
	if taskID == "" {
		this.CustomAbort(503, "task_id 为必选参数")
	}

	startPos, _ := this.GetInt64("start_postion", 0)

	endPos, _ := this.GetInt64("end_postion")
	if endPos == 0 {
		endPos, err = Redis.LLen(taskID + "_events").Result()
		if err != nil {
			this.CustomAbort(500, "redis 获取"+taskID+"长度")
		}
	}
	if endPos < startPos {
		endPos = startPos
	}

	log.Println("取日志：：：：", taskID+"_events", startPos, endPos)
	results, err := Redis.LRange(taskID+"_events", startPos, endPos).Result()
	if err != nil {
		this.CustomAbort(500, "redis 获取"+taskID+" 日志失败")
	}

	msgs := []message.Message{}
	m := message.Message{}
	for _, s := range results {
		if err := m.UnmarshalJSON([]byte(s)); err == nil {
			msgs = append(msgs, m)
		}
	}
	this.Data["json"] = msgs
	this.ServeJSON()
}
