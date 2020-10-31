package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v2"
)

const NotifyLimit int = 50

type payload struct {
	Text string `json:"text"`
}

type Submit struct {
	// Id          int     `json:"id"`
	// UserName    string `json:"user_name"`
	EpochSecond int    `json:"epoch_second"`
	ProblemId   string `json:"problem_id"`
	ContestId   string `json:"contest_id"`
	UserId      string `json:"user_id"`
	// Language    string  `json:language`
	// Point       float32 `json:point`
	// Length      int     `json:length`
	Result string `json:"result"`
	// ExecTime    int     `json:execution_time`
}
type Problem struct {
	ContestId string `json:"contest_id"`
	ProblemId string `json:"problem_id"`
}

type Param struct {
	SubmitReqUrl  string   `yaml:"submit_req_url"`
	ProblemReqUrl string   `yaml:"problem_req_url"`
	UserName      []string `yaml:"user_name"`
	WebhookUrl    string   `yaml:"webhook_url"`
	DbPath        string   `yaml:"db_path"`
}

type SubmitList []Submit

type ProblemList []Problem

func (r SubmitList) Len() int {
	return len(r)
}

func (r SubmitList) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type ByEpochSecond struct {
	SubmitList
}

func (b ByEpochSecond) Less(i, j int) bool {
	return b.SubmitList[i].EpochSecond > b.SubmitList[j].EpochSecond
}

func getUserSubmit(req_url string) (SubmitList, error) {

	res, err := http.Get(req_url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var sub_list SubmitList = []Submit{}
	err = json.Unmarshal(body, &sub_list)
	if err != nil {
		return nil, err
	}

	sort.Sort(ByEpochSecond{sub_list})
	time.Sleep(2)
	return sub_list, nil
}

func getProblemInfo(req_url string) (ProblemList, error) {

	res, err := http.Get(req_url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var problem_list ProblemList = []Problem{}
	err = json.Unmarshal(body, &problem_list)
	if err != nil {
		return nil, err
	}
	time.Sleep(2)
	return problem_list, nil
}

func readParam(yaml_path string) (Param, error) {
	buf, err := ioutil.ReadFile(yaml_path)
	p := Param{}
	if err != nil {
		return p, err
	}
	yaml.Unmarshal(buf, &p)
	return p, err
}

func initProbDb(req_url string, db_path string) error {

	DbConnection, err := sql.Open("sqlite3", db_path)
	defer DbConnection.Close()
	if err != nil {
		return err
	}

CREATE:
	cmd := `CREATE TABLE Problem(
		ContestId STRING,
		ProblemId STRING,
		Url STRING)`
	_, err = DbConnection.Exec(cmd)
	if err != nil {
		cmd = `DROP TABLE Problem`
		DbConnection.Exec(cmd)
		goto CREATE
	}

	problem_list, err := getProblemInfo(req_url)
	if err != nil {
		return err
	}
	cmd = "INSERT INTO 'Problem' ('ContestId','ProblemId','Url') VALUES(?,?,?)"
	stmt, err := DbConnection.Prepare(cmd)
	defer stmt.Close()
	if err != nil {
		return err
	}
	for _, problem := range problem_list {
		url := fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s", problem.ContestId, problem.ProblemId)
		stmt.Exec(problem.ContestId, problem.ProblemId, url)
	}

	return nil
}

func toAbsPath(ref_path string) (string, error) {
	exe_path, err := os.Executable()
	if err != nil {
		return exe_path, err
	}
	conf_path := filepath.Join(filepath.Dir(exe_path), ref_path)
	return conf_path, nil
}
func main() {

	conf_path, err := toAbsPath("conf.yaml")
	if err != nil {
		log.Fatalln(err)
	}

	p, err := readParam(conf_path)
	if err != nil {
		log.Fatalln(err)
	}
	DbPath, err := toAbsPath(p.DbPath)
	if err != nil {
		log.Fatalln(err)
	}
	var isInit = flag.Bool("init", false, "bool flag")
	flag.Parse()
	if *isInit {
		fmt.Println("init Problem DB...")
		err = initProbDb(p.ProblemReqUrl, DbPath)
	}
	//connect to db
	DbConnection, _ := sql.Open("sqlite3", DbPath)
	defer DbConnection.Close()
	// create submit table
	cmd := `CREATE TABLE IF NOT EXISTS Submit(
		Id STRING PRIMARY KEY,
		UserName STRING,
		ProblemId STRING,
		EpochSecond INT,
		Result STRING)`
	_, err = DbConnection.Exec(cmd)
	if err != nil {
		log.Fatalln(err)
	}
	var notify_list []string
	//iteration for each users
	for _, user_name := range p.UserName {
		// get sumits

		sub_list, err := getUserSubmit(p.SubmitReqUrl + user_name)
		if err != nil {
			continue
		}

		// extract past submits from tables
		cmd = fmt.Sprintf(`SELECT ProblemId,EpochSecond 
						   FROM Submit 
						   WHERE UserName="%s" ORDER BY EpochSecond DESC`, user_name)
		exist_rows, err := DbConnection.Query(cmd)
		if err != nil {
			log.Fatalln(err)
		}
		defer exist_rows.Close()

		exist_Submit := map[string]bool{}
		var last_submit_unixtime int64 = 0
		for exist_rows.Next() {
			var s Submit
			_ = exist_rows.Scan(&s.ProblemId, &s.EpochSecond)
			exist_Submit[s.ProblemId] = true
			if last_submit_unixtime == 0 {
				last_submit_unixtime = int64(s.EpochSecond)
			}
		}
		last_submit := "this is first submit"
		if last_submit_unixtime != 0 {
			last_submit = time.Unix(last_submit_unixtime, 0).Format("2006-01-02")
		}

		// filter out past Submit
		ac_sub_list := []Submit{}
		for _, sub := range sub_list {
			if _, exist := exist_Submit[sub.ProblemId]; sub.Result == "AC" && !exist {
				ac_sub_list = append(ac_sub_list, sub)
			}
		}

		notify_data := fmt.Sprintf("[%s]\nnow:%s last:%s\n新しく%d問解きました！\n",
			user_name, time.Now().Format("2006-01-02"), last_submit, len(ac_sub_list))
		// fmt.Print(notify_data)
		// if there are some new submits
		if len(ac_sub_list) > 0 {
			// tx, err := DbConnection.Begin()
			// if err != nil {
			// 	log.Fatalln(err)
			// }

			// insert new Submit only
			cmd = "INSERT INTO 'Submit' ('Id','UserName','ProblemId','EpochSecond','Result') VALUES(?,?,?,?,?)"
			stmt, err := DbConnection.Prepare(cmd)
			defer stmt.Close()
			if err != nil {
				log.Fatalln(err)
			}
			// insert!
			for cnt, sub := range ac_sub_list {
				Id := fmt.Sprintf("%s_%s", user_name, sub.ProblemId)
				_, err = stmt.Exec(Id, user_name, sub.ProblemId, sub.EpochSecond, sub.Result)
				if err != nil {
					continue
				}
				// find problem url
				cid := strings.ToLower(sub.ContestId)
				pid := strings.Split(strings.ToLower(sub.ProblemId), "_")
				cmd = fmt.Sprintf(`SELECT Url FROM Problem WHERE ContestId="%s" AND ProblemId LIKE "%%_%s"`, cid, pid[len(pid)-1])
				rows, err := DbConnection.Query(cmd)
				var problem_url = "?"
				if err == nil {
					for rows.Next() {
						rows.Scan(&problem_url)
					}
				}
				if cnt < NotifyLimit {
					// notify_data += fmt.Sprintf("%s %s:(%s)\n", time.Unix(int64(sub.EpochSecond), 0).Format("2006-01-01 15:00:05"), string(sub.ProblemId), url)
					notify_data += fmt.Sprintf("%s:(%s)\n", string(sub.ProblemId), problem_url)
				}
			}
			notify_list = append(notify_list, notify_data)
		}
	}
	//send to slack!
	for _, notify_data := range notify_list {
		pay, err := json.Marshal(payload{Text: notify_data})
		if err != nil {
			log.Fatalln(err)
		}
		resp, err := http.PostForm(p.WebhookUrl, url.Values{"payload": {string(pay)}})
		if err != nil {
			log.Fatalln(err)
		}
		defer resp.Body.Close()
		fmt.Println(notify_data)
	}
	fmt.Println("finish")
}
