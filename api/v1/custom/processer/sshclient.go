package processer

// "github.com/hnakamur/go-scp"
import (
	"bufio"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/soopsio/go-sshd/scp"
	"golang.org/x/crypto/ssh"
)

type sshClient struct {
	client *ssh.Client
}

func NewSSHClient(user, password, ip_port string) (sshClient, error) {
	PassWd := []ssh.AuthMethod{ssh.Password(password)}
	Conf := ssh.ClientConfig{User: user, Auth: PassWd,
		//需要验证服务端，不做验证返回nil就可以，点击HostKeyCallback看源码就知道了
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 10 * time.Second,
	}
	client, err := ssh.Dial("tcp", ip_port, &Conf)
	if err != nil {
		return sshClient{}, err
	}
	return sshClient{client: client}, nil
}

func (c *sshClient) ScpFile(src, dst string) error {
	return scp.NewSCP(c.client).SendFile(src, dst)
}

func (c *sshClient) remoteRun(cmd string, stdout, stderr io.Writer) error {
	if session, err := c.client.NewSession(); err == nil {

		defer session.Close()
		// SendRequest 用途参考
		// https://github.com/golang/crypto/blob/master/ssh/session.go
		// session.SendRequest("aaaa", false, []byte("bbbb"))

		// session.Stdout = os.Stdout
		// session.Stderr = os.Stderr
		// if stdout != nil {
		// 	session.Stdout = stdout
		// }

		// if stderr != nil {
		// 	session.Stderr = stderr
		// }

		stdoutPipe, err := session.StdoutPipe()
		if err != nil {
			return err
		}

		stderrPipe, err := session.StderrPipe()
		if err != nil {
			return err
		}
		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			reader := bufio.NewReader(stdoutPipe)
			buf := make([]byte, 0, 1024)
			for {
				// 读取一行数据，交给后台处理
				line, isPrefix, err := reader.ReadLine()
				if len(line) > 0 {
					buf = append(buf, line...)
					if !isPrefix {
						stdout.Write(buf)
						buf = []byte{}
					}
				}
				if err != nil {
					return
				}
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			reader := bufio.NewReader(stderrPipe)
			buf := make([]byte, 0, 1024)
			for {
				// 读取一行数据，交给后台处理
				line, isPrefix, err := reader.ReadLine()
				if len(line) > 0 {
					buf = append(buf, line...)
					if !isPrefix {
						stdout.Write(buf)
						buf = []byte{}
					}
				}
				if err != nil {
					break
				}
			}
		}()

		if err := session.Start("bash -c \"" + cmd + "\""); err != nil {
			return err
		}
		log.Println("等待运行结束", cmd)
		err = session.Wait()
		wg.Wait()
		log.Println("脚本执行完成", cmd)
		return err
		// return session.Run()

	} else {
		return err
	}
	return nil
}
