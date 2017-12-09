package processer

import (
	"encoding/json"
	"strings"

	"github.com/soopsio/mgmt/api/v1/models"
	// "github.com/tidwall/gjson"
)

// 执行任务的 worker
func Ansible(task_id, groupid string, args ...string) (string, error) {
	for _, arg := range args {
		task_args := &models.Task{}
		if err := json.Unmarshal([]byte(arg), task_args); err != nil {
			return "", err
		}

		// 根据 Inventory 信息生成密码，并返回带密码的 Inventory
		invsWithPass, _ := genInventory(task_id, task_args.Inventory, true)
		task_args.Inventory = invsWithPass

		// 解析为主机维度的任务详情
		hosttask := parseTaskToHostTask(task_id, task_args)

		switch task_args.Type {
		case "playbook":
			runPlaybook(task_args.Playbook, task_args.ExtraVars, hosttask)
		default:
			// TODO: shell/commond/script
		}
		res := "11111111"
		res = strings.Replace(res, "\x00", "", -1)
		return res, nil
	}

	return "", err
}
