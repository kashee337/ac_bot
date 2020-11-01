package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v2"
)

const (
	NotifyLimit int    = 50
	DBMS        string = "sqlite3"
)

type payload struct {
	Text string `json:"text"`
}

type Submit struct {
	UserId      string `gorm:"primary_key" json:"user_id"`
	ProblemId   string `gorm:"primary_key" json:"problem_id"`
	ContestId   string `json:"contest_id"`
	Result      string `json:"result"`
	EpochSecond int64  `json:"epoch_second"`
}
type Problem struct {
	ContestId  string `gorm:"primary_key" json:"contest_id"`
	ProblemId  string `gorm:"primary_key" json:"problem_id"`
	ProblemUrl string `json:"problem_url"`
}

type Results struct {
	Submit
	ProblemUrl string `json:"problem_url"`
}
type Conf struct {
	SubmitReqUrl  string   `yaml:"submit_req_url"`
	ProblemReqUrl string   `yaml:"problem_req_url"`
	UserId        []string `yaml:"user_id"`
	WebhookUrl    string   `yaml:"webhook_url"`
	DbPath        string   `yaml:"db_path"`
}

func getUserSubmit(req_url string) ([]Submit, error) {

	res, err := http.Get(req_url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	sub_list := []Submit{}
	err = json.Unmarshal(body, &sub_list)
	if err != nil {
		return nil, err
	}

	time.Sleep(2)
	return sub_list, nil
}

func getProblemInfo(req_url string) ([]Problem, error) {

	res, err := http.Get(req_url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	problem_list := []Problem{}
	err = json.Unmarshal(body, &problem_list)
	if err != nil {
		return nil, err
	}
	for i, problem := range problem_list {
		problem_list[i].ProblemUrl = fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s", problem.ContestId, problem.ProblemId)
	}

	time.Sleep(2)

	return problem_list, nil
}

func readConf(yaml_path string) (Conf, error) {
	buf, err := ioutil.ReadFile(yaml_path)
	p := Conf{}
	if err != nil {
		return p, err
	}
	yaml.Unmarshal(buf, &p)
	return p, err
}

func dBConnect(db_path string) *gorm.DB {
	DbConnection, err := gorm.Open(DBMS, db_path)
	if err != nil {
		log.Fatal(err)
	}
	return DbConnection
}
func initProblemDb(db_path string) {
	DbConnection := dBConnect(db_path)
	defer DbConnection.Close()

	if !(DbConnection.HasTable(&Problem{})) {
		DbConnection.CreateTable(&Problem{})
	}
}

func updateProblemDb(req_url string, db_path string) {
	DbConnection := dBConnect(db_path)
	defer DbConnection.Close()

	problem_list, err := getProblemInfo(req_url)
	if err != nil {
		log.Fatal(err)
	}

	for _, problem := range problem_list {
		DbConnection.Create(&problem)
	}
}

func initSubmitDb(db_path string) {
	DbConnection := dBConnect(db_path)
	defer DbConnection.Close()

	if !(DbConnection.HasTable(&Submit{})) {
		DbConnection.CreateTable(&Submit{})
	}
}

func updateSubmitDb(req_url string, db_path string) {
	DbConnection := dBConnect(db_path)
	defer DbConnection.Close()

	submit_list, err := getUserSubmit(req_url)
	if err != nil {
		log.Fatal(err)
	}
	for _, submit := range submit_list {
		DbConnection.Create(&submit)
	}
}
func getNewSubmitDb(user_id string, db_path string) []Results {
	DbConnection := dBConnect(db_path)
	defer DbConnection.Close()
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	unix_yesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), yesterday.Hour(), 0, 0, 0, time.Local).Unix()

	new_submits := []Results{}

	DbConnection.Table("submits").Select("submits.*,problems.problem_url").Where("user_id=? AND epoch_second> ?", user_id, unix_yesterday).
		Joins("LEFT JOIN problems ON problems.contest_id=submits.contest_id AND problems.problem_id=submits.problem_id").
		Order("epoch_second DESC").Scan(&new_submits)

	return new_submits
}
func getLastSubmit(user_id string, db_path string) Results {
	DbConnection := dBConnect(db_path)
	defer DbConnection.Close()
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	unix_yesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), yesterday.Hour(), 0, 0, 0, time.Local).Unix()

	last_submit := Results{}
	DbConnection.Table("submits").Select("submits.*,problems.*").Where("user_id=? AND epoch_second < ?", user_id, unix_yesterday).
		Joins("LEFT JOIN problems ON problems.contest_id=submits.contest_id AND problems.problem_id=submits.problem_id").
		Order("epoch_second DESC").Take(&last_submit)

	return last_submit
}

func toAbsPath(ref_path string) (string, error) {
	exe_path, err := os.Executable()
	if err != nil {
		return exe_path, err
	}
	conf_path := filepath.Join(filepath.Dir(exe_path), ref_path)
	return conf_path, nil
}

func makeNotifyData(new_submits []Results, last_submit Results) string {

	user_id := last_submit.UserId
	now := time.Now().Format("2006-01-02")
	last := time.Unix(last_submit.EpochSecond, 0).Format("2006-01-02")

	header := fmt.Sprintf("[%sさん]\nnow:%s last:%s\n新しく%d問解きました！\n", user_id, now, last, len(new_submits))
	var body string = ""
	for _, submit := range new_submits {
		body += fmt.Sprintf("%s: %s\n", submit.Submit.ProblemId, submit.ProblemUrl)
	}
	return header + body
}

func main() {

	//read config
	conf_path, err := toAbsPath("conf.yaml")
	if err != nil {
		log.Fatalln(err)
	}
	conf, err := readConf(conf_path)
	if err != nil {
		log.Fatalln(err)
	}

	DbPath, err := toAbsPath(conf.DbPath)
	if err != nil {
		log.Fatalln(err)
	}

	//init db
	initSubmitDb(DbPath)
	initProblemDb(DbPath)

	//update problem tables
	updateProblemDb(conf.ProblemReqUrl, DbPath)

	var notify_list []string
	//iteration for each users
	for _, user_id := range conf.UserId {
		updateSubmitDb(conf.SubmitReqUrl+user_id, DbPath)
		new_submits := getNewSubmitDb(user_id, DbPath)
		if len(new_submits) == 0 {
			continue
		}
		last_submit := getLastSubmit(user_id, DbPath)
		notify_list = append(notify_list, makeNotifyData(new_submits, last_submit))

	}
	//send to slack!
	for _, notify_data := range notify_list {
		data, err := json.Marshal(payload{Text: notify_data})
		if err != nil {
			log.Fatalln(err)
		}
		_, err = http.PostForm(conf.WebhookUrl, url.Values{"payload": {string(data)}})
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(notify_data)
	}
	fmt.Println("finish")
}
