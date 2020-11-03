package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kashee337/ac_bot/config"
	"github.com/kashee337/ac_bot/controller"
	"github.com/kashee337/ac_bot/sender"
	_ "github.com/mattn/go-sqlite3"
)

func toAbsPath(ref_path string) (string, error) {
	exe_path, err := os.Executable()
	if err != nil {
		return exe_path, err
	}
	conf_path := filepath.Join(filepath.Dir(exe_path), ref_path)
	return conf_path, nil
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
			fmt.Println(user_id + ":skip")
			continue
		}
		last_submit := controller.GetLastSubmit(user_id, DbPath)
		notify_list = append(notify_list, sender.MakeNotifyData(new_submits, last_submit))

	}
	//send to slack!
	sender.NotifyAC(notify_list, conf.WebhookUrl)
	fmt.Println("finish")
}
