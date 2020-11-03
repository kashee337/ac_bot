package model

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
