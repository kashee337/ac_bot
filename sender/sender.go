package sender

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/kashee337/ac_bot/controller"
	"github.com/kashee337/ac_bot/model"
)

func MakeNotifyData(new_submits []controller.Results, last_submit controller.Results) string {

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

func NotifyAC(notify_list []string, webhook_url string) {
	for _, notify_data := range notify_list {
		data, err := json.Marshal(model.Payload{Text: notify_data})
		if err != nil {
			log.Fatalln(err)
		}
		_, err = http.PostForm(webhook_url, url.Values{"payload": {string(data)}})
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(notify_data)
	}
}
