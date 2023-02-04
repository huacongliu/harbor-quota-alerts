package main

// harbor仓库配额监控脚本
// 1.获取所有带配额仓库id
// 2.通过id获取每个仓库信息并计算是否超过阈值百分比
// 3.将百分比发送至飞书报警群

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Harbor struct {
	ID           int       `json:"id"`
	Ref          Ref       `json:"ref"`
	CreationTime time.Time `json:"creation_time"`
	UpdateTime   time.Time `json:"update_time"`
	Hard         Hard      `json:"hard"`
	Used         Used      `json:"used"`
}
type Ref struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	OwnerName string `json:"owner_name"`
}
type Hard struct {
	Count   int   `json:"count"`
	Storage int64 `json:"storage"`
}
type Used struct {
	Count   int   `json:"count"`
	Storage int64 `json:"storage"`
}

var Excess = make(map[string]int)

var storageId = []int{}

// 获取仓库id
func getStorageId() {
	// 拼接执行命令行
	// -------------------修改项1-------------------
	// 账号密码,harbor地址
	var cmd0 = `curl --insecure -u "xxx:xxx" -X GET -H "Content-Type: application/json" "https://xxx.com/api/quotas/"`
	fmt.Println(cmd0)
	cmd := exec.Command("/bin/bash", "-c", cmd0)

	// 获取管道输入
	output, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("无法获取命令的标准输出管道", err.Error())
		return
	}

	// 执行Linux命令
	if err := cmd.Start(); err != nil {
		fmt.Println("Linux命令执行失败，请检查命令输入是否有误", err.Error())
		return
	}

	// 读取所有输出
	bytes, err := ioutil.ReadAll(output)
	if err != nil {
		fmt.Println("打印异常，请检查")
		return
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Wait", err.Error())
		return
	}

	// 打印请求结果
	//fmt.Printf("打印请求结果：\n%s", bytes)
	v2 := fmt.Sprintf("%s", bytes)
	//fmt.Println(v2)

	// json变量转换成数组
	list := []Harbor{}
	err = json.Unmarshal([]byte(v2), &list)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	//fmt.Print(list)
	for _, v := range list {
		hardStorage := v.Hard.Storage
		if hardStorage != -1 {
			vId := v.ID
			storageId = append(storageId, vId)
		}
	}
	fmt.Println("打印未设置配额仓库id:", storageId)
}

// 获取仓库信息
func getStorage(v int) {
	// 拼接执行命令行
	// -------------------修改项2-------------------
	// 账号密码,harbor地址
	var cmd0 = `curl --insecure -u "xxx:xxx" -X GET -H "Content-Type: application/json" "https://xxx.com/api/quotas/` + fmt.Sprintf("%v", v) + `"`
	fmt.Println(cmd0)
	cmd := exec.Command("/bin/bash", "-c", cmd0)

	// 获取管道输入
	output, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("无法获取命令的标准输出管道", err.Error())
		return
	}

	// 执行Linux命令
	if err := cmd.Start(); err != nil {
		fmt.Println("Linux命令执行失败，请检查命令输入是否有误", err.Error())
		return
	}

	// 读取所有输出
	bytes, err := ioutil.ReadAll(output)
	if err != nil {
		fmt.Println("打印异常，请检查")
		return
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Wait", err.Error())
		return
	}

	// 打印请求结果
	//fmt.Printf("%s",bytes)

	// 将二进制json变量转换成结构体
	var s Harbor
	json.Unmarshal([]byte(bytes), &s)

	//获取需要的信息
	storage_name := s.Ref.Name
	storage_used_storage := float64(s.Used.Storage) / 1024 / 1024 / 1024
	storage_hard_storage := float64(s.Hard.Storage) / 1024 / 1024 / 1024
	//fmt.Println(storage_name)
	//fmt.Println(storage_used_storage)
	//fmt.Println(storage_hard_storage)

	// 计算数据是否超过阈值并记录
	used_data := float64(storage_used_storage) / float64(storage_hard_storage) * 100
	//fmt.Println(used_data)
	if used_data > 95 {
		Excess[storage_name] = int(used_data)
	}
	//fmt.Println(Excess)
}

// 发送消息
func sendMsg(apiUrl, msg string) {
	// json
	contentType := "application/json"
	// data
	sendData := `{
    "msg_type": "text",
    "content": {"text": "` + "消息通知:" + msg + `"}
  }`
	// request
	result, err := http.Post(apiUrl, contentType, strings.NewReader(sendData))
	if err != nil {
		fmt.Printf("post failed, err:%v\n", err)
		return
	}
	defer result.Body.Close()
}

//最终发送消息
func fullStorage(storageStatus int) {
	// webhook地址
	var webhookUrl string
	// 消息内容
	var message string

	// 判断仓库是否爆满仓库
	if storageStatus == 1 {
		message = "以下仓库即将爆满,请及时清理"
		for k, v := range Excess {
			message = message + "\\n" + k + "=" + strconv.Itoa(v) + "%"
		}
		message = message + "\\n" + "<at user_id=\\\"osd_xxx\\\">所有人</at>"
	} else {
		message = "未检测到即将爆满仓库"
	}
	fmt.Println(message)

	// -------------------修改项3-------------------
	// 飞书web-hook地址
	flag.StringVar(&webhookUrl, "u", "https://open.feishu.cn/xxxxx", "飞书webhook地址")
	flag.StringVar(&message, "s", message, "需要发送的消息内容")
	flag.Parse()
	flag.Usage()
	sendMsg(webhookUrl, message)
}

func main() {
	// 1.获取带配额仓库id
	//var storage_id = [...]int{30,46,34,42,31,24,40,50,41,57,44,33,113,38,128,35}
	//fmt.Println(storage_id)
	getStorageId()

	// 2.通过id获取每个仓库信息并处理
	for _, v := range storageId {
		getStorage(v)
	}

	// 3.将百分比发送至飞书报警群
	if len(Excess) != 0 {
		fmt.Println("打印超出配额仓库id:", Excess)
		storageStatus := 1
		fullStorage(storageStatus)
	} else {
		storageStatus := 0
		fullStorage(storageStatus)
	}
}
