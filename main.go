package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/robfig/cron"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"github.com/PuerkitoBio/goquery"
	"errors"
	"strings"
)
const LOGIN_URL string = "https://hacpai.com/api/v2/login"
const CHECK_GET_URL string = "https://hacpai.com/activity/checkin"
const USER_PAGE string = "https://hacpai.com/member/"
// 超时时长 30s
const TimeOut  = 30 * 1000 * 1000 * 1000
var sign_succ bool
var user_agent string
/**
*入口
 */
func main() {
	err := godotenv.Load("conf.env")
	passwd := getMd5(os.Getenv("userPassword"))
	log.Println("userPasswd:", passwd)
	os.Setenv("passwd", passwd)
	log.Println("handle user:" + os.Getenv("userName") + "daily task")
	user_agent = os.Getenv("ua")
	log.Println("use user agent:", user_agent)
	if err != nil {
		log.Fatal("读取配置文件失败", err)
		return
	}
	sign_succ = false
	execCheck()
	cronTask()
}
// 积分信息结构体
type SignInfo struct {
	Total      string   `json:"积分总数"`// 积分总数
	Action     string   `json:"活动项"`// 操作
	Change     string   `json:"积分变动"`// 变动
	Continuous string   `json:"连续签到"`// 连续x天
}
// 定时任务
func cronTask() {
	spec := os.Getenv("checkCron")
    spec2 := os.Getenv("checkCron2")
    spec3 := os.Getenv("dailyRefresh")
	log.Println("cron task :" + spec + "begin to start!")
	c := cron.New()
	c.AddFunc(spec, execCheck)
    c.AddFunc(spec2, execCheck)
	c.AddFunc(spec3, dailyFresh)
	c.Start()
	select {}
}

func dailyFresh()  {
	log.Println("daily fresh sign_succ", sign_succ)
	sign_succ = false
}
// 获取md5
func getMd5(str string) string {
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(str))
	cipherStr := md5Ctx.Sum(nil)
	return hex.EncodeToString(cipherStr)
}
// 执行登录，签到
func execCheck() {
	log.Println("开始执行")
	if sign_succ {
		log.Println("sign succ already today.")
		return
	}
	token, err := postLogin()
	if err != nil {
		log.Println("ERR 登录失败", err)
		return
	}
	log.Println("get sym token:", token)
	url, err := visitCheckPage(token, CHECK_GET_URL)
	if err != nil {
		log.Println("获取签到url返回结果：", err)
		return
	}
	log.Print("获取签到url", url)
	// 签到
	resp, err := hacpaiHttpExec(token, url)
	if err != nil {
		log.Println("ERR 签到异常", err)
	}
	log.Println("获取结果:", resp);
	sign_succ = true
}

/**获取签到url和token
 */
func visitCheckPage(token string, url string) (string, error) {
	client := &http.Client{}
	client.Timeout = TimeOut;
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("ERR exectue url "+url+" failed,", err)
		return "", err
	}
	req.Header.Set("User-Agent", user_agent)
	req.Header.Set("Referer", "https://hacpai.com/")
	cookie := http.Cookie{Name: "symphony", Value: token, Path: "/", MaxAge: 86400}
	req.AddCookie(&cookie)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		log.Println("ERR get response failed", err)
		return "", err
	}
	dom, err := goquery.NewDocumentFromReader(resp.Body);
	if err != nil {
		log.Println("ERR 签到信息获取异常", err)
	}
	res, exsit := dom.Find("div .module__body .btn ").First().Attr("href");
	if exsit {
		has := strings.HasPrefix(res, USER_PAGE)
		if has {
			analyseDom(*dom)
			return res, errors.New("该日已签到")
		}
		return res, err
	} else {
		return "", errors.New("sign url with token not found")
	}
}

/**
 *解析dom结果
 */
func analyseDom(dom goquery.Document) (string) {
	text := dom.Find("h2 .sub-head .ft-gray").Text();
	score := dom.Find("div .module__body .btn").First().Text();
	change := dom.Find("div .module__body .vditor-reset").Text();
	signInfo := SignInfo{
		Action:     text,
		Total:      score,
		Change:     strings.TrimSpace(change),
		Continuous: ""};
	log.Println("执行返回:", signInfo)
	info, err2 := json.Marshal(signInfo);
	if err2 != nil {
		log.Println("ERR 异常", err2);
	}
	return string(info)
}

// 执行请求
func hacpaiHttpExec(token string, url string) (string, error) {
	client := &http.Client{}
	client.Timeout = TimeOut;
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("ERR exectue url "+url+" failed,", err)
		return "", err
	}
	req.Header.Set("User-Agent", user_agent)
	req.Header.Set("Referer", CHECK_GET_URL)
	cookie := http.Cookie{Name: "symphony", Value: token, Path: "/", MaxAge: 86400}
	req.AddCookie(&cookie)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		log.Println("ERR get response failed", err)
		return "", err
	}
	dom, err := goquery.NewDocumentFromReader(resp.Body);
	if err != nil {
		log.Println("ERR 签到信息获取异常", err)
		return "", err
	}
	res := analyseDom(*dom)
	return res, nil
}

// 登录hacpai
func postLogin() (string, error) {
	userData := make(map[string]interface{})
	userData["userName"] = os.Getenv("userName")
	userData["userPassword"] = os.Getenv("passwd")
	userData["captcha"] = ""
	bytesData, err := json.Marshal(userData)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	reader := bytes.NewReader(bytesData)
	request, err := http.NewRequest("POST", LOGIN_URL, reader)
	if err != nil {
		log.Println(err.Error())
		return "", err
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set("User-Agent", user_agent)
	client := http.Client{}
	client.Timeout = TimeOut;
	resp, err := client.Do(request)
	if err != nil {
		log.Println(err.Error())
		return "", err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return "", err
	}
	respData := make(map[string]interface{})
	json.Unmarshal(respBytes, &respData)
	log.Println(respData)
	return respData["token"].(string), nil
}
