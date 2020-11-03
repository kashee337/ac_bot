package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/kashee337/ac_bot/config"
	"github.com/kashee337/ac_bot/controller"
	_ "github.com/mattn/go-sqlite3"
)

type payload struct {
	Text string `json:"text"`
}

func toAbsPath(ref_path string) (string, error) {
	exe_path, err := os.Executable()
	if err != nil {
		return exe_path, err
	}
	conf_path := filepath.Join(filepath.Dir(exe_path), ref_path)
	return conf_path, nil
}

func makeNotifyData(new_submits []controller.Results, last_submit controller.Results) string {

	user_id := last_submit.Submit.UserId
	now := time.Now().Format("2006-01-02")
	last := time.Unix(last_submit.Submit.EpochSecond, 0).Format("2006-01-02")

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
	conf, err := config.ReadConf(conf_path)
	if err != nil {
		log.Fatalln(err)
	}

	DbPath, err := toAbsPath(conf.DbPath)
	if err != nil {
		log.Fatalln(err)
	}

	//init db
	controller.InitSubmitDb(DbPath)
	controller.InitProblemDb(DbPath)

	//update problem tables
	controller.UpdateProblemDb(conf.ProblemReqUrl, DbPath)

	var notify_list []string
	//iteration for each users
	for _, user_id := range conf.UserId {
		controller.UpdateSubmitDb(conf.SubmitReqUrl+user_id, DbPath)
		new_submits := controller.GetNewSubmitDb(user_id, DbPath)
		if len(new_submits) == 0 {
			continue
		}
		last_submit := controller.GetLastSubmit(user_id, DbPath)
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
