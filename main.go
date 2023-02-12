package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/ini.v1"
	aliyun_ddns "local_ddns/aliyun"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	runPath := getRunPath()
	if runPath == "" {
		fmt.Println("启动路径获取失败")
		return
	}
	c := "/"
	if runtime.GOOS == "windows" {
		c = "\\"
	}
	if !fileExists(runPath+c+"config"+c+"access.ini") ||
		!fileExists(runPath+c+"config"+c+"domain.json") {
		fmt.Println("配置文件缺失")
		return
	}

	key, er := getAccessKey(runPath + c + "config" + c + "access.ini")
	if er != nil {
		fmt.Println("AccessKey数据获取失败")
		return
	}
	ptr, err := aliyun_ddns.New(key)
	if err != nil {
		fmt.Println("初始化失败，", err.Error())
		return
	}

	domainList := getJsonData(runPath + c + "config" + c + "domain.json")
	if len(domainList) == 0 {
		fmt.Println("解析json数据失败")
		return
	}
	for _, value := range domainList {
		err = ptr.AddDomains(value)
		if err != nil {
			num := 1
			for err != nil {
				fmt.Println("加入域名信息失败，", err.Error(), "尝试次数:", num)
				num += 1
				time.Sleep(10 * time.Second)
				err = ptr.AddDomains(value)
			}
		}
	}

	ptr.Start()
}

// 获取程序启动路径
func getRunPath() string {
	ex, err := os.Executable()
	if err != nil {
		return ""
	}
	exPath := filepath.Dir(ex)
	return exPath
}

// 判断文件/文件夹是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

// 获取accessKey
func getAccessKey(path string) (aliyun_ddns.AccessKey, error) {
	key := aliyun_ddns.AccessKey{}
	cfg, err := ini.Load(path)
	if err != nil {
		return key, err
	}
	key.Id = cfg.Section("AccessKey").Key("Id").String()
	key.KeySecret = cfg.Section("AccessKey").Key("KeySecret").String()
	key.Endpoint = cfg.Section("AccessKey").Key("Endpoint").String()

	return key, nil
}

// 获取json内部数据
func getJsonData(path string) []aliyun_ddns.Domain {
	var domainList []aliyun_ddns.Domain
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("json文件打开失败")
		return domainList
	}
	err = json.Unmarshal(data, &domainList)
	if err != nil {
		fmt.Println("json转结构体失败")
		return domainList
	}
	return domainList
}
