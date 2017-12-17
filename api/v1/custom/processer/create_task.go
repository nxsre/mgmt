package processer

import (
	"encoding/json"
	"strings"
	"time"

	machinerytasks "github.com/RichardKnop/machinery/v1/tasks"
	middleware "github.com/go-openapi/runtime/middleware"
	"github.com/soopsio/mgmt/api/v1/custom/pipe/message"
	apimodels "github.com/soopsio/mgmt/api/v1/models"
	"github.com/soopsio/mgmt/api/v1/restapi/operations/tasks"
	"github.com/soopsio/mgmt/controllers" // 使用其中定义的 TaskID
)

func CreateTask(p tasks.CreateTaskParams) middleware.Responder {
	jb, err := json.Marshal(p.Task)
	if err != nil {
		return tasks.NewCreateTaskDefault(500)
	}
	task_sign := machinerytasks.NewSignature("ansible", nil)
	task_sign.Args = []machinerytasks.Arg{
		// 注册当前任务 UUID 和 GroupUUID，做起始的两个参数传入 Task，task 处理时，从第三个参数开始为任务的真实参数
		{
			Type:  "string",
			Value: task_sign.UUID,
		},
		{
			Type:  "string",
			Value: task_sign.GroupUUID,
		},
		{
			Type:  "string",
			Value: string(jb),
		},
	}
	asyncResult, err := server.SendTask(task_sign)
	taskState := asyncResult.GetState()
	restasks := &apimodels.TaskState{}
	restasks.Status = taskState.State
	restasks.Taskid = asyncResult.Signature.UUID
	restasks.Version = strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
	tasksRes := tasks.NewCreateTaskOK()

	t := strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
	// 初始化状态消息
	collStatus(&message.Message{
		TaskID: task_sign.UUID,
		Type:   "taskstatus",
		Content: message.MsgContent{
			Status:    taskState.State,
			TimeStamp: t,
			Sequnce:   t,
		},
	})

	// 推送到内部接口和外部接口的list
	tid := controllers.Taskid{Ansible_Task_ID: task_sign.UUID}
	tidjb, _ := json.Marshal(tid)
	Redis.RPush("ansible_task_id_channel_longpoll", string(tidjb)).Result()
	Redis.RPush("ansible_task_id_channel", task_sign.UUID).Result()

	return tasksRes.WithPayload(restasks)
}
