package core

type Comics struct {
	ID    int      `db:"id"`
	URL   string   `db:"url"`
	Words []string `db:"words"`
}
