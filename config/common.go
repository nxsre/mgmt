package config

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/astaxie/beego/config"
)

var (
	inifile config.Configer
	err     error
	sep     = string(filepath.Separator)
)

// 判断是否从命令行传入参数
func isFlag(name string) bool {
	for _, v := range os.Args {
		if v == "-"+name {
			return true
		}
	}
	return false
}

func init() {
	// 默认配置文件为 conf/app.conf
	conf := flag.String("cfg", "conf/app.conf", "app config file")

	// 当前执行文件为路径
	file, _ := exec.LookPath(os.Args[0])

	// 获取可执行文件的绝对路径
	path, _ := filepath.Abs(file)

	path = filepath.Dir(path)

	// 如果可执行文件在 Temp 下，则使用查找源码相对路径下的配置文件
	if strings.HasPrefix(path, os.TempDir()) {
		_, file, _, _ = runtime.Caller(0)
		path = getParentDirectory(filepath.Dir(file))
	}

	// 配置文件路径为
	filename := path + sep + *conf

	// 如果命令行指定了配置文件路径，则使用指定的路径覆盖默认配置文件路径
	if isFlag("cfg") {
		filename = *conf
	}

	inifile, err = loadConfig(filename)
	if err != nil {
		fmt.Println("打开配置文件失败", err)
		os.Exit(2)
	}
}

func substr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}

func getParentDirectory(dirctory string) string {
	return substr(dirctory, 0, strings.LastIndex(dirctory, sep))
}

func loadConfig(filename string) (config.Configer, error) {
	return config.NewConfig("ini", filename)
}

func GetConfig() config.Configer {
	return inifile
}
