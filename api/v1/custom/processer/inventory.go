package processer

import (
	"github.com/mitchellh/mapstructure"
	"github.com/revel/cron"
	"github.com/soopsio/mgmt/api/v1/models"
	"go.uber.org/config"

	"fmt"
	"log"
	"math/big"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/tv42/base58"
)

type VaultConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	SSHPath  string `yaml:"sshpath"`
}

var (
	vaultconfig      *VaultConfig
	crontab          *cron.Cron
	forks            string // ansible: specify number of parallel processes to use
	ansible_verbose  string
	redisChanneltype string
)

func Auth(c *vaultapi.Client, m map[string]string) (string, error) {
	var data struct {
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
		Mount    string `mapstructure:"mount"`
		Method   string `mapstructure:"method"`
		Passcode string `mapstructure:"passcode"`
	}
	if err := mapstructure.WeakDecode(m, &data); err != nil {
		return "", err
	}

	if data.Username == "" {
		return "", fmt.Errorf("'username' must be specified")
	}
	if data.Password == "" {
		return "", fmt.Errorf("'password' must be specified")
	}
	if data.Mount == "" {
		data.Mount = "userpass"
	}

	options := map[string]interface{}{
		"password": data.Password,
	}
	if data.Method != "" {
		options["method"] = data.Method
	}
	if data.Passcode != "" {
		options["passcode"] = data.Passcode
	}

	path := fmt.Sprintf("auth/%s/login/%s", data.Mount, data.Username)
	secret, err := c.Logical().Write(path, options)
	if err != nil {
		return "", err
	}
	if secret == nil {
		return "", fmt.Errorf("empty response from credential provider")
	}

	return secret.Auth.ClientToken, nil
}

// 申请otp秘钥 无系统调用
func reqOtp(host, user string) (string, error) {
	if strings.HasPrefix(user, "task_") {
		// userArr := strings.Split(user, "#")
		// user = userArr[1]
		user = "vault"
	}
	apiconfig := &vaultapi.Config{Address: vaultconfig.URL}
	apiclient, err := vaultapi.NewClient(apiconfig)
	if err != nil {
		return "", err
	}
	usermap := map[string]string{
		"username": vaultconfig.Username,
		"password": vaultconfig.Password,
	}
	token, err := Auth(apiclient, usermap)
	if err != nil {
		return "", err
	}
	apiclient.SetToken(token)
	datamap := make(map[string]interface{})
	datamap["username"] = user
	datamap["ip"] = host
	secret, err := Write(apiclient, vaultconfig.SSHPath, datamap)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(secret.Data["key"]), nil
}

// 写入数据到vault
func Write(client *vaultapi.Client, path string, data map[string]interface{}) (*vaultapi.Secret, error) {
	secret, err := client.Logical().Write(path, data)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// genInventory 生成 Inventory
func genInventory(task_id string, inDatas models.InventoryDatas, pass bool) (models.InventoryDatas, error) {
	// 生成 Inventory

	/*	{
		"testgroup": {
				"hosts": [
						"172.168.88.100",
						"172.168.88.101"
				],
				"vars": {
						"ansible_ssh_user": "",
						"ansible_ssh_port": 2022,
						"ansible_ssh_private_key_file": "",
						"example_variable": ""
				}
		},
		"_meta": {
				"hostvars": {
						"172.168.88.101": {
								"ansible_ssh_user": "vault",
								"ansible_ssh_pass": "269479be-b0ed-6f18-28c8-a7d40dbfbe34",
								"host_specific_var": "aaa=bbb"
						},
						"172.168.88.100": {
								"ansible_ssh_user": "vault",
								"ansible_ssh_pass": "68aa6def-be2e-1507-cb78-87f4f0d893c2",
								"host_specific_var": "aaa=bbb"
						}
				}
		}
	}*/

	// 定义 dnyInventory
	Inventory := make(map[string]interface{})

	groupvars := make(map[string]interface{})

	meta := make(map[string]interface{})
	taskUser := task_id
	// 压缩uuid,将用户名转换为 task_b58_<22bit> 格式
	if strings.HasPrefix(taskUser, "task_") {
		num := new(big.Int)
		taskUser = strings.Replace(strings.Replace(taskUser, "task_", "", 1), "-", "", -1)
		if _, ok := num.SetString(taskUser, 16); !ok {
			log.Fatalf("not a number: %s", taskUser)
		}
		taskUser = "task_b58_" + string(base58.EncodeBig(nil, num))
	}

	// 从 inDatas 获取 _meta 信息
	hostvars := make(map[string]map[string]interface{})
	if _meta, ok := inDatas["_meta"]; ok {
		if hvars, ok := _meta["hostvars"]; ok {
			if hvs, ok := hvars.(map[string]interface{}); ok {
				for ip, vars := range hvs {
					if vs, ok := vars.(map[string]interface{}); ok {
						hostvars[ip] = vs
					}
				}
			}
		}
	}
	// fmt.Println("拼好的 hostvars:", hostvars)
	for k, v := range inDatas {
		fmt.Println(k, v)
		if k != "_meta" { // key 不是 _meta 则为 hostgroup 组名
			if hosts, ok := v["hosts"]; ok {
				if hs, ok := hosts.([]interface{}); ok {
					for _, ip := range hs {
						if i, ok := ip.(string); ok {
							hostvar := map[string]interface{}{}
							// 如果有已存在的主机变量，直接复制给 hostvar
							if hvar, ok := hostvars[i]; ok {
								hostvar = hvar
							}
							hostvar["ansible_ssh_user"] = taskUser //+ "#vault"

							if pass {
								// 获取 otp token
								// otpKey, err := reqOtp(i, taskUser)
								otpKey := "passssssssssss"
								// if err != nil {
								// 	// return "", err
								// }
								hostvar["ansible_ssh_pass"] = otpKey
							}
							hostvars[i] = hostvar
						}
					}
				} else {
					log.Println("主机格式不正确：", hosts)
				}
			} else {
				fmt.Println("没有主机列表")
			}

			if gvars, ok := v["vars"]; ok {
				if vars, ok := gvars.(map[string]interface{}); ok {
					groupvars = vars
				}
			}
			groupvars["ansible_ssh_port"] = 2022

			// fmt.Println(v["hosts"])
			hgroup := make(map[string]interface{})
			hgroup["vars"] = groupvars

			hgroup["hosts"] = v["hosts"]
			Inventory[k] = hgroup
		}
	}
	meta["hostvars"] = hostvars
	Inventory["_meta"] = meta

	p, _ := config.NewStaticProvider(&Inventory)
	invs := models.InventoryDatas{}
	p.Get("").Populate(&invs)
	return invs, nil
}
