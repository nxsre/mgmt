package processer

import (
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/flosch/pongo2"
	"github.com/google/shlex"
	"github.com/google/uuid"
	"github.com/jaytaylor/html2text"
	"github.com/soopsio/mgmt/api/v1/custom/pipe/message"
	apimodels "github.com/soopsio/mgmt/api/v1/models"
	"go.uber.org/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 运行 Playbook
func runPlaybook(pb string, extvars apimodels.ExtraVars, hostinfo HostTasks) {
	pbpath := filepath.Join("/data/svn/ansible", pb)
	hostinfo, _ = parsePlaybook(pbpath, extvars, hostinfo)
	hostinfo.runCommand()
	logger.Info("debug", zap.Any("hostinfo", hostinfo), zap.Any("pb", pbpath), zap.Any("extravars", extvars))
}

// 解析 Playbook
func parsePlaybook(pbpath string, extvars apimodels.ExtraVars, hostinfo HostTasks) (HostTasks, error) {
	// 加载 Playbook
	pbProvider, err := config.NewYAMLProviderFromFiles(pbpath)
	if err != nil {
		return HostTasks{}, err
	}
	// 获取所有属性
	val := pbProvider.Get("")
	if !val.HasValue() {
		return HostTasks{}, errors.New("获取 Playbook 内容失败")
	}

	// 遍历主机列表
	for host, hosttask := range hostinfo.TaskInfo {
		// 合并主机变量和扩展变量，如果冲突，则主机变量覆盖扩展变量
		pongo2ctx := extvars2Context(extvars)
		for k, v := range hosttask.vars {
			pongo2ctx[k] = v
		}

		pbhosts := []interface{}{}
		err = val.Populate(&pbhosts)
		for _, pbhost := range pbhosts {
			pbhostProvider, _ := config.NewStaticProvider(pbhost)
			if pbhostProvider.Get("tasks").HasValue() {
				tasks := []map[string]interface{}{}
				if err := pbhostProvider.Get("tasks").Populate(&tasks); err != nil {
					return HostTasks{}, errors.New("获取 tasks 内容失败")
				}

				for _, task := range tasks {
					c := command{}
					for k, v := range task {
						var content string
						if tv, ok := v.(string); !ok {
							logger.Error("value not string")
						} else {
							content = tv
							// 加载模板文件
							tpl, _ := pongo2.FromString(content)
							// 渲染模板
							bs, _ := tpl.ExecuteBytes(pongo2ctx)
							content, _ = html2text.FromString(string(bs))
						}
						switch k {
						case "name":
							c.name = content
						case "script":
							c.content = content
							c.cmdType = "script"
						case "shell":
							c.content = content
							c.cmdType = "shell"
						default:
							// TODO: 其它模块支持
						}
					}
					hostinfo.TaskInfo[host].cmd = &c
				}
			}
		}
	}

	return hostinfo, nil
}

// script
type command struct {
	name    string
	content string
	cmdType string
}

// ExtraVars 转为 pongo2.Context
func extvars2Context(extvars apimodels.ExtraVars) pongo2.Context {
	pangoCtx := map[string]interface{}{}
	for k, v := range extvars {
		pangoCtx[k] = v
	}
	return pangoCtx
}

// 解析变量
func vars2Map(v interface{}) map[string]interface{} {
	vmap := map[string]interface{}{}
	if varmap, ok := v.(map[interface{}]interface{}); ok {
		for k, v := range varmap {
			if sk, ok := k.(string); ok {
				vmap[sk] = v
			}
		}
	}

	if varmap, ok := v.(map[string]interface{}); ok {
		for k, v := range varmap {
			vmap[k] = v
		}
	}
	return vmap
}

// 存储主机的任务信息
type HostTask struct {
	vars map[string]interface{}
	cmd  *command
}

type HostTasks struct {
	TaskInfo map[string]*HostTask
	TaskID   string
	Stdout   *Wstd
	Stderr   *Wstd
	ExitCode int
}

// 解析 Inventory
func parseTaskToHostTask(task_id string, t *apimodels.Task) HostTasks {
	invs := t.Inventory
	var hoststask = HostTasks{TaskInfo: map[string]*HostTask{}, TaskID: task_id}

	// 解析全局 _meta
	if _meta, ok := invs["_meta"]; ok {
		// 解析主机变量
		if hostvars, ok := _meta["hostvars"]; ok {
			for k, v := range vars2Map(hostvars) {
				if _, ok := hoststask.TaskInfo[k]; !ok {
					hoststask.TaskInfo[k] = &HostTask{}
				}
				hoststask.TaskInfo[k].vars = vars2Map(v)
			}
		}

		// TODO: 其它类型 meta 解析
	}

	// 解析主机组信息
	for group, inv := range invs {
		switch group {
		case "_meta":
			// 跳过 _meta 信息（用来设置变量等，非主机组）
			continue
		default:
			// 除 _meta 外，其他均为主机组名称及组内主机列表
			groupvars := map[string]interface{}{}
			_ = groupvars
			if vars, ok := inv["vars"]; ok {
				groupvars = vars2Map(vars)
			}

			if hosts, ok := inv["hosts"]; ok {
				if hostarr, ok := hosts.([]interface{}); ok {
					for _, host := range hostarr {
						if k, ok := host.(string); ok {
							if _, ok := hoststask.TaskInfo[k]; !ok {
								hoststask.TaskInfo[k] = &HostTask{}
							}
							// 附加组变量
							for vk, vv := range groupvars {
								hoststask.TaskInfo[k].vars[vk] = vv
							}
						}
					}
				}

			}
		}
	}

	return hoststask
}

// 执行命令
func (ht *HostTasks) runCommand() {
	t := strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
	msg := &message.Message{
		TaskID: ht.TaskID,
		Type:   "taskstatus",
		Content: message.MsgContent{
			Status:    "RUNNING",
			TimeStamp: t,
			Sequnce:   t,
		},
	}
	collChildStatus(msg)
	// 多主机并行
	wg := &sync.WaitGroup{}
	for h, task := range ht.TaskInfo {
		ht.Stderr = &Wstd{level: zapcore.ErrorLevel, task_id: ht.TaskID, host: h, task_name: task.cmd.name}
		ht.Stdout = &Wstd{level: zapcore.InfoLevel, task_id: ht.TaskID, host: h, task_name: task.cmd.name}
		wg.Add(1)
		go func() {
			defer wg.Done()
			cstatus := "RUNNING"
			t = strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
			msg = &message.Message{
				TaskID: ht.TaskID,
				Type:   "childstatus",
				Content: message.MsgContent{
					Status:    cstatus,
					TimeStamp: t,
					Sequnce:   t,
					TaskName:  task.cmd.name,
					Host:      h,
				},
			}
			collChildStatus(msg)

			defer func() {
				t = strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
				msg = &message.Message{
					TaskID: ht.TaskID,
					Type:   "childstatus",
					Content: message.MsgContent{
						Status:    cstatus,
						TimeStamp: t,
						Sequnce:   t,
						TaskName:  task.cmd.name,
						Host:      h,
					},
				}
				collChildStatus(msg)
			}()

			t, _ := config.NewStaticProvider(task.vars)
			user := t.Get("ansible_ssh_user").String()
			pass := t.Get("ansible_ssh_pass").String()
			addr := h + ":" + t.Get("ansible_ssh_port").String()
			sshclient, err := NewSSHClient(user, pass, addr)
			if err != nil {
				log.Println(err)
				cstatus = "UNREACHABLE"
				return
			}
			defer sshclient.client.Close()
			shles, err := shlex.Split(task.cmd.content)
			if err != nil {
				log.Println(err)
				cstatus = "FAILURE"
				return
			}
			if len(shles) > 0 {
				log.Println("执行命令:", task.cmd.content, task.cmd.name)
				if task.cmd.cmdType == "script" {
					remoteTmpdir := "/tmp/.ansible_tmp/" + uuid.New().String()
					defer func() {
						sshclient.remoteRun("rm -rf "+remoteTmpdir, ht.Stdout, ht.Stderr)
					}()
					remoteScript := filepath.Join(remoteTmpdir, filepath.Base(shles[0]))
					if err := sshclient.remoteRun("mkdir -p "+remoteTmpdir, ht.Stdout, ht.Stderr); err != nil {
						logger.Error("mkdir -p "+remoteTmpdir+" failed", zap.Error(err))
						cstatus = "FAILURE"
						return
					}
					if err := sshclient.ScpFile(shles[0], remoteScript); err != nil {
						logger.Error("SCP "+shles[0]+" failed", zap.Error(err))
						cstatus = "FAILURE"
						return
					}
					sshclient.remoteRun("chmod +x "+remoteScript, ht.Stdout, ht.Stderr)
					remoteCmd := remoteScript
					if len(shles) > 1 {
						remoteCmd += " " + strings.Join(shles[1:], " ")
					}
					if err := sshclient.remoteRun(remoteCmd, ht.Stdout, ht.Stderr); err != nil {
						logger.Error("Run "+remoteCmd+" failed", zap.Error(err))
						cstatus = "FAILURE"
						return
					}

					cstatus = "SUCCESSANDCHANGED"
				}
			}

		}()
	}

	wg.Wait()
	t = strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
	msg = &message.Message{
		TaskID: ht.TaskID,
		Type:   "taskstatus",
		Content: message.MsgContent{
			Status:    "FINISHED",
			TimeStamp: t,
			Sequnce:   t,
		},
	}
	collChildStatus(msg)
	logger.Info("finished msg!!!", zap.String("task_id", ht.TaskID), zap.Bool("endflag", true))
}

// 写日志
type Wstd struct {
	level     zapcore.Level
	host      string
	task_id   string
	task_name string
}

func (w Wstd) Write(p []byte) (n int, err error) {
	switch w.level {
	case zapcore.InfoLevel:
		logger.Info(string(p), zap.String("task_id", w.task_id), zap.String("host", w.host), zap.String("task_name", w.task_name))
	case zapcore.ErrorLevel:
		logger.Error(string(p), zap.String("task_id", w.task_id), zap.String("host", w.host), zap.String("task_name", w.task_name))
	default:
		log.Println("unknow level")
	}
	return len(p), nil
}
