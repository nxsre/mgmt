package processer

import (
	"log"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	// "encoding/json"
	// "fmt"
	// "time"
	// middleware "github.com/go-openapi/runtime/middleware"
	// "github.com/go-redis/redis"
	"github.com/soopsio/mgmt/api/v1/custom/pipe/message"
	// "github.com/soopsio/mgmt/api/v1/models"
	// "github.com/soopsio/mgmt/api/v1/restapi/operations/tasks"
)

func init() {
	go processMsg()
}

func processMsg() {
	for {
		if strs := Redis.BLPop(1*time.Second, "ansible_task_id_channel"); len(strs.Val()) > 1 {
			log.Println("TASKID::::", strs.Val()[1])
			go func() {
				for {
					if msgstrs := Redis.BLPop(1*time.Second, strs.Val()[1]+"_logs"); len(msgstrs.Val()) > 1 {
						msgstr := msgstrs.Val()[1]
						if gmsg := gjson.Parse(msgstr).Get("message"); gmsg.Exists() {
							// 判断是否结束，如果结束，退出循环
							if end := gmsg.Get("endflag"); end.Bool() {
								log.Println(strs.Val()[1], " 运行结束，退出日志监听")
								return
							}
							t := strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
							msg := &message.Message{
								TaskID: gmsg.Get("task_id").String(),
								Type:   "log",
								Content: message.MsgContent{
									Host:      gmsg.Get("host").String(),
									Msg:       gmsg.Get("msg").String(),
									TimeStamp: t,
									Sequnce:   t,
									TaskName:  gmsg.Get("task_name").String(),
								},
							}
							msgbs, _ := msg.MarshalJSON()
							Redis.RPush(strs.Val()[1]+"_events", string(msgbs)).Result()
						}
					}
				}
			}()
		}
	}
}
