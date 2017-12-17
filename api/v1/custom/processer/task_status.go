package processer

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"

	middleware "github.com/go-openapi/runtime/middleware"
	"github.com/go-redis/redis"
	"github.com/soopsio/mgmt/api/v1/custom/pipe/message"
	"github.com/soopsio/mgmt/api/v1/models"
	"github.com/soopsio/mgmt/api/v1/restapi/operations/tasks"
)

// collStatus 搜集子任务状态
func collStatus(m *message.Message) {
	lock.Lock()

	// 推一条状态到 redis 的 task_events
	// msgbs, _ := m.MarshalJSON()
	// Redis.RPush(m.TaskID+"_events", string(msgbs)).Result()
	// 改用 logger 同步状态到redis
	logger.Info("", zap.String("task_id", m.TaskID), zap.String("status", m.Content.Status), zap.String("type", m.Type), zap.String("host", m.Content.Host), zap.String("task_name", m.Content.TaskName))

	taskstatusdetail := &models.TaskState{}
	taskstatusdetail.Taskid = m.TaskID
	taskstatusdetail.Version = m.Content.TimeStamp

	keyname := "ansible_task_id:" + m.TaskID + ":status"
	res, err := Redis.Get(keyname).Result()
	if m.Type == "childstatus" && m.Content.Status != "" {
		childstatus := make(models.ChildStatus)
		childstatus[m.Content.TaskName] = m.Content.Status
		// redis.Nil 表示redis结果为空
		if err == redis.Nil {
			if res == "" {
				taskstatusdetail = &models.TaskState{
					Taskid: m.TaskID,
					Childstatus: map[string]models.ChildStatus{
						m.Content.Host: childstatus,
					},
					Status: "RUNNING",
				}

				log.Println("redis 中不存在此任务状态，设置子任务状态为 RUNNING")
			}
		} else {
			err := json.Unmarshal([]byte(res), taskstatusdetail)
			if err != nil {
				fmt.Println("反序列化失败:", res, err)
			}
			if _, ok := taskstatusdetail.Childstatus[m.Content.Host]; ok {
				taskstatusdetail.Childstatus[m.Content.Host][m.Content.TaskName] = m.Content.Status
			} else {
				taskstatusdetail.Childstatus = map[string]models.ChildStatus{}
				taskstatusdetail.Childstatus[m.Content.Host] = childstatus
			}
		}
		jb, err := json.Marshal(taskstatusdetail)
		if err == nil {
			res, err = Redis.Set(keyname, string(jb), 86400*time.Second).Result()
			fmt.Println("set", res, err)
		}
	}
	if m.Type == "taskstatus" {
		if err != redis.Nil {
			err := json.Unmarshal([]byte(res), taskstatusdetail)
			if err != nil {
				fmt.Println("反序列化失败:", res, err)
			}
		}

		taskstatusdetail.Status = m.Content.Status
		jb, err := json.Marshal(taskstatusdetail)
		if err == nil {
			res, err = Redis.Set(keyname, string(jb), 86400*time.Second).Result()
			fmt.Println("set", res, err)
		}
	}
	lock.Unlock()
}

func GetTaskStatus(task_id string) middleware.Responder {
	keyname := "ansible_task_id:" + task_id + ":status"
	res, err := Redis.Get(keyname).Result()
	if err != nil {
		return tasks.NewGetTaskDefault(500).WithPayload(
			&models.Error{
				Code:    5501,
				Fields:  "",
				Message: err.Error(),
			})
	}

	statusDtails := models.TaskState{}
	if res != "" {
		err = json.Unmarshal([]byte(res), &statusDtails)
	}

	if err != nil {
		return tasks.NewGetTaskDefault(500).WithPayload(
			&models.Error{
				Code:    5502,
				Fields:  "",
				Message: err.Error(),
			})
	}
	return tasks.NewGetTaskOK().WithPayload(&statusDtails)
}
