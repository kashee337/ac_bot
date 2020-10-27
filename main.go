package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sort"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v2"
)

type payload struct {
	Text string `json:"text"`
}

type Submits struct {
	// Id          int     `json:"id"`
	UserName    string `json:"user_name"`
	EpochSecond int    `json:"epoch_second"`
	ProblemId   string `json:"problem_id"`
	// ContestId   string  `json:contest_id`
	UserId string `json:user_id`
	// Language    string  `json:language`
	// Point       float32 `json:point`
	// Length      int     `json:length`
	Result string `json:result`
	// ExecTime    int     `json:execution_time`
}

type Param struct {
	BaseUrl    string   `yaml:"base_url"`
	UserName   []string `yaml:"user_name"`
	WebhookUrl string   `yaml:"webhook_url"`
	DbPath     string   `yaml:"db_path"`
}

type SubmitsList []Submits

func (r SubmitsList) Len() int {
	return len(r)
}

func (r SubmitsList) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type ByEpochSecond struct {
	SubmitsList
}

func (b ByEpochSecond) Less(i, j int) bool {
	return b.SubmitsList[i].EpochSecond < b.SubmitsList[j].EpochSecond
}

func getUserSubmits(base_url string, username string) SubmitsList {
	req_url := base_url + username

	res, err := http.Get(req_url)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var sub_list SubmitsList = []Submits{}
	err = json.Unmarshal(body, &sub_list)
	if err != nil {
		log.Fatalln(err)
	}

	sort.Sort(ByEpochSecond{sub_list})
	return sub_list
}

func readParam(yaml_path string) Param {
	buf, err := ioutil.ReadFile("./test.yaml")
	if err != nil {
		log.Fatalln(err)
	}
	p := Param{}
	yaml.Unmarshal(buf, &p)
	return p
}

func main() {

	p := readParam("./conf.yaml")

	//connect to db
	DbConnection, _ := sql.Open("sqlite3", p.DbPath)
	defer DbConnection.Close()
	cmd := `CREATE TABLE IF NOT EXISTS submits(
		UserName STRING,
		ProblemId STRING,
		EpochSecond INT,
		Result STRING)`
	_, err := DbConnection.Exec(cmd)
	if err != nil {
		log.Fatalln(err)
	}

	for _, user_name := range p.UserName {

		sub_list := getUserSubmits(p.BaseUrl, user_name)

		// extract from tables
		exist_rows, err := DbConnection.Query(`SELECT * FROM submits`)
		if err != nil {
			log.Fatalln(err)
		}
		defer exist_rows.Close()
		exist_submits := map[string]bool{}
		for exist_rows.Next() {
			var s Submits
			_ = exist_rows.Scan(&s.ProblemId, &s.EpochSecond, &s.Result)
			exist_submits[s.ProblemId] = true
		}
		// filter out old submits
		ac_sub_list := []Submits{}
		for i := 0; i < len(sub_list); i++ {
			if _, exist := exist_submits[sub_list[i].ProblemId]; sub_list[i].Result == "AC" && !exist {
				ac_sub_list = append(ac_sub_list, sub_list[i])
			}
		}
		notify_data := fmt.Sprintf("[%s]new submits num is %d\n", user_name, len(ac_sub_list))
		fmt.Print(notify_data)
		if len(ac_sub_list) > 0 {
			// tx, err := DbConnection.Begin()
			// if err != nil {
			// 	log.Fatalln(err)
			// }

			// insert new submits only
			cmd = "INSERT INTO 'submits' ('UserName','ProblemId','EpochSecond','Result') VALUES(?,?,?,?)"
			stmt, err := DbConnection.Prepare(cmd)
			defer stmt.Close()
			if err != nil {
				log.Fatalln(err)
			}
			for i := 0; i < len(ac_sub_list); i++ {
				stmt.Exec(user_name, ac_sub_list[i].ProblemId, ac_sub_list[i].EpochSecond, ac_sub_list[i].Result)
				notify_data += string(ac_sub_list[i].ProblemId) + ","
			}
			fmt.Println(notify_data)

			pay, err := json.Marshal(payload{Text: notify_data})
			if err != nil {
				log.Fatalln(err)
			}
			resp, err := http.PostForm(p.WebhookUrl, url.Values{"payload": {string(pay)}})
			if err != nil {
				log.Fatalln(err)
			}
			defer resp.Body.Close()
		}
	}
}
