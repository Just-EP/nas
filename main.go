package main

import (
	"fmt"
	"github.com/pkg/sftp"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"io"
	"os"
	"path"
	"time"
)

func main() {
	// 创建一个新的 cron 调度器
	c := cron.New(cron.WithSeconds()) // v3 需要启用秒的支持
	var params = loadConfig()
	// 使用 cron 表达式注册定时任务
	_, err := c.AddFunc(params.Cron, func() {
		go downloadAndShowTime(params)
	})

	if err != nil {
		fmt.Println("Error adding cron function:", err)
		return
	}

	// 启动 cron 调度器
	c.Start()

	// 由于 cron 会在后台运行，我们需要阻塞主线程来保持程序一直运行
	select {}
}

type ConnectionParam struct {
	User        string   `yaml:"user"`
	Password    string   `yaml:"password"`
	Host        string   `yaml:"host"`
	Port        int      `yaml:"port"`
	RemoteFiles []string `yaml:"remoteFiles"`
	Cron        string   `yaml:"c"`
}
type Config struct {
	Connection ConnectionParam `yaml:"connection"`
}

func loadConfig() ConnectionParam {
	viper.SetConfigName("config.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("读取配置失败: %s\n", err))
	}

	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		panic(fmt.Errorf("解析配置失败: %s", err))
	}
	fmt.Println("配置文件内容:", viper.AllSettings())
	return config.Connection
}

func downloadAndShowTime(params ConnectionParam) {
	fmt.Println("Start download at", time.Now().Format(time.DateTime))
	downloadRemote(params)
}

func downloadRemote(params ConnectionParam) {
	client, err := connect(params)
	if err != nil {
		return
	}
	_ = downloadFiles(client, params.RemoteFiles, "./")
}

func connect(param ConnectionParam) (*sftp.Client, error) {
	config := &ssh.ClientConfig{
		User: param.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(param.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", param.Host, param.Port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func downloadFiles(client *sftp.Client, remoteFiles []string, localDir string) error {
	for _, remoteFile := range remoteFiles {
		// 打开远程文件
		srcFile, err := client.Open(remoteFile)
		if err != nil {
			return fmt.Errorf("failed to open remote file %s: %v", remoteFile, err)
		}

		// 创建本地文件
		localFilePath := path.Join(localDir, path.Base(remoteFile))
		dstFile, err := os.Create(localFilePath)
		if err != nil {
			_ = srcFile.Close() // 记得手动关闭已打开的远程文件
			return fmt.Errorf("failed to create local file %s: %v", localFilePath, err)
		}

		// 手动关闭文件
		// 注意：要确保在函数返回前关闭文件，防止资源泄漏
		err = copyFile(srcFile, dstFile)
		if err != nil {
			_ = srcFile.Close()
			_ = dstFile.Close()
			return fmt.Errorf("failed to copy file %s to %s: %v", remoteFile, localFilePath, err)
		}

		// 关闭文件
		_ = srcFile.Close()
		_ = dstFile.Close()

		// 输出下载成功信息
		fmt.Printf("Downloaded %s to %s\n", remoteFile, localFilePath)
	}
	return nil
}

// 用于复制文件内容的辅助函数
func copyFile(src *sftp.File, dst *os.File) error {
	_, err := io.Copy(dst, src)
	return err
}
