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

type reqRes struct {
	err error
	rc  bool
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
		log.Println("等待运行结束", c.client.LocalAddr(), cmd)

		go func() {
			resch := make(chan reqRes)

			// 后台发送 keepalived 探测包
			session, err := c.client.NewSession()
			if err != nil {
				c.client.Close()
				return
			}

			counter := 0
			timer := time.NewTimer(10 * time.Second)
			for {
				time.Sleep(1 * time.Second)
				go func() {
					b, err := session.SendRequest("keepalive@golang.org", false, nil)
					resch <- reqRes{
						err: err,
						rc:  b,
					}
				}()

				select {
				case res := <-resch:
					if res.err != nil {
						if res.err == io.EOF {
							log.Println("连接关闭：", c.client.LocalAddr())
						} else {
							log.Println("发送心跳包失败", c.client.LocalAddr(), err)
							c.client.Close()
						}
						return
					} else {
						if res.rc {
							// log.Println("心跳正常", c.client.LocalAddr())
						} else {
							log.Println("心跳异常", c.client.LocalAddr())
							c.client.Close()
						}
					}
				case <-timer.C:
					if counter >= 3 {
						log.Println("心跳超时", c.client.LocalAddr())
						c.client.Close()
						return
					}
					counter++
				}
				timer.Reset(time.Second * 10)
			}
		}()
		err = session.Wait()
		wg.Wait()
		log.Println("脚本执行完成", c.client.LocalAddr(), cmd)
		return err
		// return session.Run()

	} else {
		return err
	}
	return nil
}
