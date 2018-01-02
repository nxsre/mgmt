package processer

import (
	"errors"
	"fmt"
	"log"
	"os"
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
	var err error
	pbpath := filepath.Join("/data/svn/ansible", pb)
	ts := strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
	var collTaskStatus = func(status,m string) {
		msg := &message.Message{
			TaskID: hostinfo.TaskID,
			Type:   "taskstatus",
			Content: message.MsgContent{
				Status:    status,
				TimeStamp: ts,
				Sequnce:   ts,
				Msg: m,
			},
		}
		collStatus(msg)
	}

	hostinfo, err = parsePlaybook(pbpath, extvars, hostinfo)
	if err!=nil{
		collTaskStatus("FAILED",err.Error())
		return
	}
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
						case "command":
							c.content = content
							c.cmdType = "command"
						default:
							// TODO: 其它模块支持
						}
					}
					hostinfo.TaskInfo[host].cmd = append(hostinfo.TaskInfo[host].cmd, c)
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
	cmd  []command
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
	ts := strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
	var collTaskStatus = func(status string) {
		msg := &message.Message{
			TaskID: ht.TaskID,
			Type:   "taskstatus",
			Content: message.MsgContent{
				Status:    status,
				TimeStamp: ts,
				Sequnce:   ts,
			},
		}
		collStatus(msg)
	}

	collTaskStatus("RUNNING")
	// 多主机并行
	wg := &sync.WaitGroup{}
	for h, task := range ht.TaskInfo {
		fmt.Println("TaskInfo:::", task.cmd)

		t, _ := config.NewStaticProvider(task.vars)
		user := t.Get("ansible_ssh_user").String()
		pass := t.Get("ansible_ssh_pass").String()
		addr := h + ":" + t.Get("ansible_ssh_port").String()
		sshclient, err := NewSSHClient(user, pass, addr)
		if err != nil {
			log.Println(err)
			msg := &message.Message{
				TaskID: ht.TaskID,
				Type:   "childstatus",
				Content: message.MsgContent{
					Status:    "UNREACHABLE",
					TimeStamp: ts,
					Sequnce:   ts,
					Host:      h,
					Msg:       err.Error(),
				},
			}
			collStatus(msg)
			continue
		}
		defer func() {
			if err := sshclient.client.Close(); err != nil {
				log.Println("关闭 %s 连接失败:%v", sshclient.client.RemoteAddr(), err)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, c := range task.cmd {
				ht.Stderr = &Wstd{level: zapcore.ErrorLevel, task_id: ht.TaskID, host: h, task_name: c.name}
				ht.Stdout = &Wstd{level: zapcore.InfoLevel, task_id: ht.TaskID, host: h, task_name: c.name}

				cstatus := "RUNNING"
				ts := strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
				msg := &message.Message{
					TaskID: ht.TaskID,
					Type:   "childstatus",
					Content: message.MsgContent{
						Status:    cstatus,
						TimeStamp: ts,
						Sequnce:   ts,
						TaskName:  c.name,
						Host:      h,
					},
				}
				collStatus(msg)

				childStatus := func(status string) {
					ts = strings.Replace(time.Now().Format("20060102150405.0000"), ".", "", -1)
					msg = &message.Message{
						TaskID: ht.TaskID,
						Type:   "childstatus",
						Content: message.MsgContent{
							Status:    status,
							TimeStamp: ts,
							Sequnce:   ts,
							TaskName:  c.name,
							Host:      h,
						},
					}
					collStatus(msg)
				}

				shles, err := shlex.Split(c.content)
				if err != nil {
					log.Println(err)
					childStatus("FAILURE")
					return
				}

				log.Println(shles)
				if len(shles) > 0 {
					log.Println("执行命令:", c.content, c.name)
					if c.cmdType == "script" {
						remoteTmpdir := "/tmp/.ansible_tmp/" + ht.TaskID + string(os.PathSeparator) + uuid.New().String()

						remoteScript := filepath.Join(remoteTmpdir, filepath.Base(shles[0]))
						if err := sshclient.remoteRun("mkdir -p "+remoteTmpdir, ht.Stdout, ht.Stderr); err != nil {
							logger.Error("mkdir -p "+remoteTmpdir+" failed", zap.Error(err))
							childStatus("FAILURE")
							return
						}
						if err := sshclient.ScpFile(shles[0], remoteScript); err != nil {
							logger.Error("SCP "+shles[0]+" failed", zap.Error(err))
							childStatus("FAILURE")
							return
						}
						sshclient.remoteRun("chmod +x "+remoteScript, ht.Stdout, ht.Stderr)
						remoteCmd := remoteScript
						if len(shles) > 1 {
							// Join shles 会把原参数中的单引号、双引号去掉
							// remoteCmd += " " + strings.Join(shles[1:], " ")
							// 改为直接截取 c.content
							remoteCmd += " " + c.content[len(shles[0])+1:]
						}
						if err := sshclient.remoteRun(remoteCmd, ht.Stdout, ht.Stderr); err != nil {
							logger.Error("Run "+remoteCmd+" failed", zap.Error(err))
							childStatus("FAILURE")
							return
						}

						if err := sshclient.remoteRun("rm -rf "+"/tmp/.ansible_tmp/"+ht.TaskID, ht.Stdout, ht.Stderr); err != nil {
							log.Printf("删除临时文件 %s 失败: %v", remoteTmpdir, err)
						}
					}
					if c.cmdType == "command" {
						fmt.Println("开始执行", c.content)
						if err := sshclient.remoteRun(c.content, ht.Stdout, ht.Stderr); err != nil {
							logger.Error("Run "+c.content+" failed", zap.Error(err))
							childStatus("FAILURE")
							return
						}
						fmt.Println("执行完毕,SUCCESSANDCHANGED", c.content)
					}

					// TODO: support shell
					childStatus("SUCCESSANDCHANGED")
				}

			}
		}()
	}

	wg.Wait()
	collTaskStatus("FINISHED")
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
		logger.Info(string(p), zap.String("task_id", w.task_id), zap.String("type", "log"), zap.String("host", w.host), zap.String("task_name", w.task_name))
	case zapcore.ErrorLevel:
		logger.Error(string(p), zap.String("task_id", w.task_id), zap.String("type", "log"), zap.String("host", w.host), zap.String("task_name", w.task_name))
	default:
		log.Println("unknow level")
	}
	return len(p), nil
}
