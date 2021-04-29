package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/kashee337/ac_bot/model"
)

const (
	DBMS string = "sqlite3"
)

type Results struct {
	model.Submit
	ProblemUrl string `json:"problem_url"`
}

func GetUserSubmit(req_url string) ([]model.Submit, error) {

	res, err := http.Get(req_url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	sub_list := []model.Submit{}
	err = json.Unmarshal(body, &sub_list)
	if err != nil {
		return nil, err
	}

	time.Sleep(5)

	return sub_list, nil
}

func GetProblemInfo(req_url string) ([]model.Problem, error) {

	res, err := http.Get(req_url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	problem_list := []model.Problem{}
	err = json.Unmarshal(body, &problem_list)
	if err != nil {
		return nil, err
	}
	for i, problem := range problem_list {
		problem_list[i].ProblemUrl = fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s", problem.ContestId, problem.ProblemId)
	}

	time.Sleep(5)

	return problem_list, nil
}

func DBConnect(db_path string) *gorm.DB {
	DbConnection, err := gorm.Open(DBMS, db_path)
	if err != nil {
		log.Fatal(err)
	}
	return DbConnection
}

func InitProblemDb(db_path string) {
	DbConnection := DBConnect(db_path)
	defer DbConnection.Close()

	if !(DbConnection.HasTable(&model.Problem{})) {
		DbConnection.CreateTable(&model.Problem{})
	}
}

func UpdateProblemDb(req_url string, db_path string) {
	DbConnection := DBConnect(db_path)
	defer DbConnection.Close()

	problem_list, err := GetProblemInfo(req_url)
	if err != nil {
		log.Fatal(err)
	}

	for _, problem := range problem_list {
		DbConnection.Where(model.Problem{ProblemId:problem.ProblemId}).FirstOrCreate(&problem)
	}
}

func InitSubmitDb(db_path string) {
	DbConnection := DBConnect(db_path)
	defer DbConnection.Close()

	if !(DbConnection.HasTable(&model.Submit{})) {
		DbConnection.CreateTable(&model.Submit{})
	}
}

func UpdateSubmitDb(req_url string, db_path string) {
	DbConnection := DBConnect(db_path)
	defer DbConnection.Close()

	submit_list, err := GetUserSubmit(req_url)
	if err != nil {
		log.Fatal(err)
	}
	for _, submit := range submit_list {
		if submit.Result == "AC" {
			DbConnection.Where(model.Submit{UserId:submit.UserId,ProblemId:submit.ProblemId}).FirstOrCreate(&submit)
		}
	}
}

func GetNewSubmitDb(user_id string, db_path string) []Results {
	DbConnection := DBConnect(db_path)
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

func GetLastSubmit(user_id string, db_path string) Results {
	DbConnection := DBConnect(db_path)
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
