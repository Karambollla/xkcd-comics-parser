package core

type ServiceStatus string

const (
	StatusRunning ServiceStatus = "running"
	StatusIdle    ServiceStatus = "idle"
)

type DBStats struct {
	WordsTotal    int `db:"words_total"`
	WordsUnique   int `db:"words_unique"`
	ComicsFetched int `db:"comics_fetched"`
}

type ServiceStats struct {
	DBStats
	ComicsTotal int
}

type Comics struct {
	ID    int      `db:"id"`
	URL   string   `db:"url"`
	Words []string `db:"words"`
}

type XKCDInfo struct {
	ID          int    `json:"num"`
	URL         string `json:"img"`
	Description string
}
