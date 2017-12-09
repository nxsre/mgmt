package pipe

import (
	"fmt"
	"log"
	"strings"
	"time"

	// sarama "github.com/Shopify/sarama"

	"github.com/soopsio/mgmt/api/v1/custom/pipe/message"
	"gopkg.in/redis.v5"
)

var (
	Redis *redis.Client
)

func init() {

	go processTaskPipe()
}

// processTaskPipe 处理管道数据（任务状态、日志等）
func processTaskPipe() {
	for {
		scmd := Redis.BLPop(60*time.Second, "ansible_task_id_channel")
		if len(scmd.Val()) < 2 {
			continue
		}
		task_id := scmd.Val()[1]
		log.Println("获取到 TaskID:", task_id)
		// sendKafka("ansible_task_id_channel", `{"ansible_task_id":"`+strings.TrimPrefix(task_id, "ansible_task_id:")+`"}`)
		// taskIDManager.Publish("ansible_task_id_channel", `{"ansible_task_id":"`+strings.TrimPrefix(task_id, "ansible_task_id:")+`"}`)
		Redis.RPush("ansible_task_id_channel_longpoll", `{"ansible_task_id":"`+strings.TrimPrefix(task_id, "ansible_task_id:")+`"}`)

		go func(tid string) {
			// 空值统计
			nullCount := 0
			for {
				scmd := Redis.BRPop(60*time.Second, tid)
				if len(scmd.Val()) < 2 {
					// 连续 1个小时无日志退出订阅
					if nullCount > 60 {
						return
					}
					nullCount++
					continue
				}
				nullCount = 0
				msg := scmd.Val()[1]
				m := message.Message{}
				if err := m.UnmarshalJSON([]byte(msg)); err != nil {
					fmt.Println(err)
				}

				collChildStatus(&m)
				// if longerr == nil {
				// 	if err := longpollManager.Publish(m.TaskID, msg); err != nil {
				// 		log.Println("longpollManager 推送失败", err)
				// 	}
				// }

				Redis.RPush(m.TaskID+"_logs", msg).Result()
				Redis.Expire(m.TaskID+"_logs", 1800*time.Second).Result()

				// 发送kafka
				// sendKafka(m.TaskID, msg)
				if m.Type == "taskstatus" && m.Content.Status != "" {
					fmt.Println(m.TaskID, "执行完毕")
					return
				}
			}
		}(task_id)

	}

}
