package aliyun_ddns

import (
	"errors"
	"fmt"
	alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"net"
	"strings"
	"time"
)

// AccessKey 阿里云访问密钥与域名
type AccessKey struct {
	Id        string
	KeySecret string
	Endpoint  string
}

// Domain 域名相关结构体
type Domain struct {
	recordId string //阿里云域名修改时所需要的id，将根据Name自动查询到
	value    string //将修改为的值

	Name    string //域名后缀
	Type    string //解析类型，一般的ipv4为A
	RR      string //域名前缀
	NetCard string //监测网卡名
}

type aliyun struct {
	accessKey  AccessKey
	client     *alidns20150109.Client
	DomainList []Domain
}

// New 构造函数，保证按要求构建aliyun结构体
func New(key AccessKey) (*aliyun, error) {
	ptr := &aliyun{}
	ptr.accessKey = key
	er := ptr.createClient()
	if er != nil {
		return nil, er
	}

	return ptr, nil
}

// createClient 使用AccessKey初始化账号
func (ptr *aliyun) createClient() error {
	config := &openapi.Config{
		AccessKeyId:     &ptr.accessKey.Id,
		AccessKeySecret: &ptr.accessKey.KeySecret,
	}
	// 访问的域名
	config.Endpoint = tea.String(ptr.accessKey.Endpoint)
	ptr.client = &alidns20150109.Client{}
	var err error
	ptr.client, err = alidns20150109.NewClient(config)
	return err
}

// AddDomains 添加将要监控的域名信息
func (ptr *aliyun) AddDomains(domain Domain) error {
	if domain.Name == "" || domain.RR == "" || domain.Type == "" || domain.NetCard == "" {
		return errors.New("参数未填写完整")
	}
	if ptr.GetNetCardIP(domain.NetCard) == "" {
		return errors.New("网卡名称错误，无法检测到此网卡")
	}
	er := ptr.searchRecordId(&domain)
	if er != nil {
		return er
	}
	ptr.DomainList = append(ptr.DomainList, domain)

	return nil
}

// searchRecordId 查询RecordId
func (ptr *aliyun) searchRecordId(domain *Domain) error {
	describeDomainRecordsRequest := &alidns20150109.DescribeDomainRecordsRequest{
		DomainName: tea.String(domain.Name),
		RRKeyWord:  tea.String(domain.RR),
	}
	runtime := &util.RuntimeOptions{}
	tryErr := func() (_e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				_e = r
			}
		}()
		request, _err := ptr.client.DescribeDomainRecordsWithOptions(describeDomainRecordsRequest, runtime)
		if _err != nil {
			return _err
		}
		requestStr := request.Body.String()
		if strings.Index(requestStr, "DomainRecords") == -1 {
			return errors.New(requestStr)
		}
		domain.recordId = *request.Body.DomainRecords.Record[0].RecordId

		return nil
	}()

	if tryErr != nil {
		var err = &tea.SDKError{}
		if _t, ok := tryErr.(*tea.SDKError); ok {
			err = _t
		} else {
			err.Message = tea.String(tryErr.Error())
		}
		_, _err := util.AssertAsString(err.Message)
		if _err != nil {
			return _err
		}
	}

	return nil
}

// JudgeChange 判断当前域名是否需要修改解析
func (ptr *aliyun) JudgeChange(domain *Domain, currIp string) (bool, error) {
	if domain.value == currIp {
		return false, nil
	}
	err := ptr.updateDomainRecord(domain, currIp)
	if err != nil {
		if domain.value == "" {
			domain.value = currIp
			//因为本程序不保存所修改的记录，所以每一次启动后此值都为空。都将会触发一次更新解析，如果云记录的数据与当前ip相等则
			//updateDomainRecord函数将会返回错误，这个错误是不需要处理的
			return false, nil
		}
		return true, err
	}

	domain.value = currIp
	return true, nil
}

// updateDomainRecord 修改指定域名的解析
func (ptr *aliyun) updateDomainRecord(domain *Domain, newIp string) error {
	updateDomainRecordRequest := &alidns20150109.UpdateDomainRecordRequest{
		RecordId: tea.String(domain.recordId),
		Type:     tea.String(domain.Type),
		Value:    tea.String(newIp),
		RR:       tea.String(domain.RR),
	}
	runtime := &util.RuntimeOptions{}
	tryErr := func() (_e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				_e = r
			}
		}()
		re, _err := ptr.client.UpdateDomainRecordWithOptions(updateDomainRecordRequest, runtime)
		if _err != nil {
			return _err
		}
		jsonStr := re.Body.String()
		if strings.Index(jsonStr, "Message") != -1 {
			return errors.New(jsonStr)
		}

		return nil
	}()

	if tryErr != nil {
		var err = &tea.SDKError{}
		if _t, ok := tryErr.(*tea.SDKError); ok {
			err = _t
		} else {
			err.Message = tea.String(tryErr.Error())
		}
		_, _err := util.AssertAsString(err.Message)
		if _err != nil {
			return _err
		}
	}
	return nil
}

// GetNetCardIP 获取指定网卡ip
func (ptr *aliyun) GetNetCardIP(name string) string {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for i := 0; i < len(netInterfaces); i++ {
		if (netInterfaces[i].Flags & net.FlagUp) != 0 {
			addrs, _ := netInterfaces[i].Addrs()

			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						if netInterfaces[i].Name == name {
							return ipnet.IP.String()
						}
					}
				}
			}
		}
	}

	return ""
}

// 查询云解析记录
func (ptr *aliyun) searchAliyunIp(recordId string) string {
	describeDomainRecordInfoRequest := &alidns20150109.DescribeDomainRecordInfoRequest{
		RecordId: tea.String(recordId),
	}
	runtime := &util.RuntimeOptions{}
	var ip *string
	tryErr := func(re *string) (_e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				_e = r
			}
		}()
		request, _err := ptr.client.DescribeDomainRecordInfoWithOptions(describeDomainRecordInfoRequest, runtime)
		if _err != nil {
			return _err
		}
		ip := *request.Body.Value
		re = &ip
		return nil
	}(ip)

	if tryErr != nil {
		var error = &tea.SDKError{}
		if _t, ok := tryErr.(*tea.SDKError); ok {
			error = _t
		} else {
			error.Message = tea.String(tryErr.Error())
		}
		_, _err := util.AssertAsString(error.Message)
		if _err != nil {
			return ""
		}
	}
	return *ip
}

// Start 检测域名与ip是否对应
func (ptr *aliyun) Start() {
	ticker1 := time.NewTicker(30 * time.Second)
	defer ticker1.Stop()
	for {
		<-ticker1.C
		for index, _ := range ptr.DomainList {
			value := &ptr.DomainList[index]

			oldIp := value.value
			currIp := ptr.GetNetCardIP(value.NetCard)
			state, err := ptr.JudgeChange(value, currIp)
			if err != nil {
				fmt.Println("修改域名解析失败，NetCard:", value.NetCard, "RR:", value.RR, "old ip:", oldIp,
					"new ip:", currIp, "recordId:", value.recordId)
				continue
			}
			if state {
				fmt.Println("尝试修改解析，RR:", value.RR, "old ip:", oldIp, "new ip:", currIp, "recordId:",
					value.recordId)
				if ptr.searchAliyunIp(value.recordId) == currIp {
					fmt.Println("检测修改成功")
				} else {
					fmt.Println("检测修改失败，开始重置ip")
					value.value = oldIp
				}
			}
		}
	}
}
