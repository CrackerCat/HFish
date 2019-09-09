package report

import (
	"HFish/core/dbUtil"
	"time"
	"HFish/utils/log"
	"HFish/utils/ip"
	"strconv"
	"HFish/utils/try"
	"strings"
	"HFish/core/alert"
	"HFish/utils/conf"
	"HFish/core/pool"
	"sync"
	"github.com/panjf2000/ants"
)

type HFishInfo struct {
	id      string
	model   string
	project string
	typex   string
	agent   string
	ip      string
	country string
	region  string
	city    string
	info    string
	time    string
}

var (
	wg    sync.WaitGroup
	poolX *ants.Pool

	wgUpdate    sync.WaitGroup
	poolUpdateX *ants.Pool
)

func init() {
	wg, poolX = pool.New(10)
	defer poolX.Release()

	wgUpdate, poolUpdateX = pool.New(10)
	defer poolUpdateX.Release()
}

// 通知模块
func alertx(id string, model string, typex string, projectName string, agent string, ipx string, country string, region string, city string, infox string, timex string) {
	wg.Add(1)
	poolX.Submit(func() {
		time.Sleep(time.Second * 2)

		// 邮件通知
		alert.AlertMail(model, typex, agent, ipx, country, region, city, infox, &wg)

		// WebHook
		alert.AlertWebHook(id, model, typex, projectName, agent, ipx, country, region, city, infox, timex, &wg)

		// 大数据展示
		//alert.AlertDataWs(model, typex, projectName, agent, ipx, country, region, city, time)
	})
}

// 上报 集群 状态
func ReportAgentStatus(agentName string, agentIp string, webStatus string, deepStatus string, sshStatus string, redisStatus string, mysqlStatus string, httpStatus string, telnetStatus string, ftpStatus string, memCacheStatus string, plugStatus string) {
	_, err := dbUtil.DB().Table("hfish_colony").Data(map[string]interface{}{
		"agent_name":       agentName,
		"agent_ip":         agentIp,
		"web_status":       webStatus,
		"deep_status":      deepStatus,
		"ssh_status":       sshStatus,
		"redis_status":     redisStatus,
		"mysql_status":     mysqlStatus,
		"http_status":      httpStatus,
		"telnet_status":    telnetStatus,
		"ftp_status":       ftpStatus,
		"mem_cache_status": memCacheStatus,
		"plug_status":      plugStatus,
		"last_update_time": time.Now().Format("2006-01-02 15:04:05"),
	}).InsertGetId()

	if err != nil {
		// 如果异常，代表触发了唯一索引，直接走更新操作
		_, err := dbUtil.DB().
			Table("hfish_colony").Data(map[string]interface{}{
			"agent_ip":         agentIp,
			"web_status":       webStatus,
			"deep_status":      deepStatus,
			"ssh_status":       sshStatus,
			"redis_status":     redisStatus,
			"mysql_status":     mysqlStatus,
			"http_status":      httpStatus,
			"telnet_status":    telnetStatus,
			"ftp_status":       ftpStatus,
			"mem_cache_status": memCacheStatus,
			"plug_status":      plugStatus,
			"last_update_time": time.Now().Format("2006-01-02 15:04:05"),
		}).Where("agent_name", agentName).Update()

		if err != nil {
			log.Pr("HFish", "127.0.0.1", "更新集群信息失败", err)
		}
	}
}

// 判断是否为白名单IP
func isWhiteIp(ip string) bool {
	var isWhite = false

	try.Try(func() {
		result, err := dbUtil.DB().Table("hfish_setting").Fields("status", "info").Where("type", "=", "whiteIp").First()

		if err != nil {
			log.Pr("HFish", "127.0.0.1", "获取白名单IP失败", err)
		}

		status := strconv.FormatInt(result["status"].(int64), 10)

		// 判断是否启用通知
		if status == "1" {
			info := result["info"]
			ipArr := strings.Split(info.(string), "&&")

			for _, val := range ipArr {
				if (ip == val) {
					isWhite = true
				}
			}
		}

	}).Catch(func() {
	})

	return isWhite
}

// 通用的插入
func insertInfo(typex string, projectName string, agent string, ipx string, country string, region string, city string, info string) int64 {

	id, err := dbUtil.DB().Table("hfish_info").Data(map[string]interface{}{
		"type":         typex,
		"project_name": projectName,
		"agent":        agent,
		"ip":           ipx,
		"country":      country,
		"region":       region,
		"city":         city,
		"info":         info,
		"create_time":  time.Now().Format("2006-01-02 15:04:05"),
	}).InsertGetId()

	if err != nil {
		log.Pr("HFish", "127.0.0.1", "插入上钩信息失败", err)
	}

	return id
}

// 更新
func updateInfoCore(id string, info string) {
	time.Sleep(time.Second * 2)

	try.Try(func() {
		var sql string

		// 此处为了兼容 Mysql + Sqlite
		dbType := conf.Get("admin", "db_type")

		if dbType == "sqlite" {
			sql = `
				UPDATE hfish_info
				SET info = info||?
				WHERE
					id = ?;
				`
		} else if dbType == "mysql" {
			sql = `
				UPDATE hfish_info
				SET info = CONCAT(info, ?)
				WHERE
					id = ?;
				`
		}

		_, err := dbUtil.DB().Execute(sql, info, id)

		if err != nil {
			log.Pr("HFish", "127.0.0.1", "更新上钩信息失败", err)
		}

		wgUpdate.Done()
	}).Catch(func() {
		wgUpdate.Done()
	})
}

// 通用的更新
func updateInfo(id string, info string) {
	wgUpdate.Add(1)
	poolUpdateX.Submit(func() {
		updateInfoCore(id, info)
	})
}

// 上报 WEB
func ReportWeb(projectName string, agent string, ipx string, info string) {
	// IP 不在白名单，进行上报
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("WEB", projectName, agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "WEB", projectName, agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// 上报 暗网 WEB
func ReportDeepWeb(projectName string, agent string, ipx string, info string) {
	// IP 不在白名单，进行上报
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("DEEP", projectName, agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "DEEP", projectName, agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// 上报 蜜罐插件
func ReportPlugWeb(projectName string, agent string, ipx string, info string) {
	// IP 不在白名单，进行上报
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("PLUG", projectName, agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "PLUG", projectName, agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// 上报 SSH
func ReportSSH(ipx string, agent string, info string) int64 {
	defer func() {
		if err := recover(); err != nil {
			log.Pr("HFish", "127.0.0.1", "执行SSH插入失败", err)
		}
	}()

	// IP 不在白名单，进行上报
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("SSH", "SSH蜜罐", agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "SSH", "SSH蜜罐", agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
		return id
	}
	return 0
}

// 更新 SSH 操作
func ReportUpdateSSH(id string, info string) {
	defer func() {
		if err := recover(); err != nil {
			log.Pr("HFish", "127.0.0.1", "执行SSH更新失败", err)
		}
	}()

	if (id != "0") {
		go updateInfo(id, info)
		go alertx(id, "update", "SSH", "SSH蜜罐", "", "", "", "", "", info, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// 上报 Redis
func ReportRedis(ipx string, agent string, info string) int64 {
	// IP 不在白名单，进行上报
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("REDIS", "Redis蜜罐", agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "REDIS", "Redis蜜罐", agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
		return id
	}
	return 0
}

// 更新 Redis 操作
func ReportUpdateRedis(id string, info string) {
	if (id != "0") {
		go updateInfo(id, info)
		go alertx(id, "update", "REDIS", "Redis蜜罐", "", "", "", "", "", info, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// 上报 Mysql
func ReportMysql(ipx string, agent string, info string) int64 {
	// IP 不在白名单，进行上报
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("MYSQL", "Mysql蜜罐", agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "MYSQL", "Mysql蜜罐", agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
		return id
	}
	return 0
}

// 更新 Mysql 操作
func ReportUpdateMysql(id string, info string) {
	if (id != "0") {
		go updateInfo(id, info)
		go alertx(id, "update", "MYSQL", "Mysql蜜罐", "", "", "", "", "", info, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// 上报 FTP
func ReportFTP(ipx string, agent string, info string) {
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("FTP", "FTP蜜罐", agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "FTP", "FTP蜜罐", agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// 上报 Telnet
func ReportTelnet(ipx string, agent string, info string) int64 {
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("TELNET", "Telnet蜜罐", agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "TELNET", "Telnet蜜罐", agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
		return id
	}
	return 0
}

// 更新 Telnet 操作
func ReportUpdateTelnet(id string, info string) {
	if (id != "0") {
		go updateInfo(id, info)
		go alertx(id, "update", "TELNET", "Telnet蜜罐", "", "", "", "", "", info, time.Now().Format("2006-01-02 15:04:05"))
	}
}

// 上报 MemCache
func ReportMemCche(ipx string, agent string, info string) int64 {
	if (isWhiteIp(ipx) == false) {
		country, region, city := ip.GetIp(ipx)
		id := insertInfo("MEMCACHE", "MemCache蜜罐", agent, ipx, country, region, city, info)
		go alertx(strconv.FormatInt(id, 10), "new", "MEMCACHE", "MemCache蜜罐", agent, ipx, country, region, city, info, time.Now().Format("2006-01-02 15:04:05"))
		return id
	}
	return 0
}

// 更新 MemCache 操作
func ReportUpdateMemCche(id string, info string) {
	if (id != "0") {
		go updateInfo(id, info)
		go alertx(id, "update", "MEMCACHE", "MemCache蜜罐", "", "", "", "", "", info, time.Now().Format("2006-01-02 15:04:05"))
	}
}
