package rest

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Karambollla/course/api/adapters/rest/middleware"
	"github.com/Karambollla/course/api/core"
)

//go:embed templates/*.html static/*.css
var webAssets embed.FS

const tokenCookieName = "token"

var pageTemplates = template.Must(template.New("pages").ParseFS(webAssets, "templates/*.html"))

type SearchResultView struct {
	ID    int
	URL   string
	Score int
}

type SearchPageData struct {
	Title   string
	Phrase  string
	Limit   int
	Scope   string
	Results []SearchResultView
	Total   int
	Notice  string
	Error   string
}

type AdminPageData struct {
	Title       string
	LoggedIn    bool
	Notice      string
	Error       string
	Stats       *core.UpdateStats
	Status      string
	LoginAction string
}

func renderPage(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return pageTemplates.ExecuteTemplate(w, name, data)
}

func parseLimit(value string) (int, error) {
	if value == "" {
		return minLim, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf("invalid limit")
	}
	return limit, nil
}

func renderHome(w http.ResponseWriter, data SearchPageData) {
	if data.Title == "" {
		data.Title = "xkcd searcher"
	}
	if err := renderPage(w, "home.html", data); err != nil {
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}

func renderAdmin(w http.ResponseWriter, data AdminPageData) {
	if data.Title == "" {
		data.Title = "Панель админа"
	}
	if data.LoginAction == "" {
		data.LoginAction = "/admin/login"
	}
	if err := renderPage(w, "admin.html", data); err != nil {
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}

func NewStaticHandler() http.Handler {
	sub, err := fs.Sub(webAssets, "static")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}

func NewSearchPageHandler(log *slog.Logger, searcher core.Searcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		phrase := strings.TrimSpace(r.URL.Query().Get("phrase"))
		scope := strings.TrimSpace(r.URL.Query().Get("scope"))
		if scope == "" {
			scope = "search"
		}

		limit, err := parseLimit(r.URL.Query().Get("limit"))
		if err != nil {
			renderHome(w, SearchPageData{Error: err.Error(), Phrase: phrase, Scope: scope, Limit: minLim})
			return
		}

		data := SearchPageData{
			Title:  "xkcd searcher",
			Phrase: phrase,
			Limit:  limit,
			Scope:  scope,
		}

		if phrase == "" {
			renderHome(w, data)
			return
		}

		var (
			comics []core.Comics
			total  int
		)

		if scope == "index" {
			comics, total, err = searcher.SearchIndex(r.Context(), phrase, limit)
		} else {
			comics, total, err = searcher.Search(r.Context(), phrase, limit)
		}
		if err != nil {
			log.Error("web search failed", "error", err)
			data.Error = "поиск временно недоступен"
			renderHome(w, data)
			return
		}

		data.Total = total
		data.Results = make([]SearchResultView, 0, len(comics))
		for _, comic := range comics {
			data.Results = append(data.Results, SearchResultView{ID: comic.ID, URL: comic.URL, Score: comic.Score})
		}
		if total == 0 {
			data.Notice = "Комиксов не найдено"
		}
		renderHome(w, data)
	}
}

func buildAdminData(r *http.Request, verifier middleware.TokenVerifier, updater core.Updater) AdminPageData {
	data := AdminPageData{}
	token := middleware.TokenFromRequest(r)
	if token == "" || verifier.Verify(token) != nil {
		return data
	}

	data.LoggedIn = true
	if stats, err := updater.Stats(r.Context()); err == nil {
		data.Stats = &stats
	}
	if status, err := updater.Status(r.Context()); err == nil {
		data.Status = string(status)
	}
	return data
}

func NewAdminPageHandler(log *slog.Logger, verifier middleware.TokenVerifier, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := buildAdminData(r, verifier, updater)
		data.Notice = r.URL.Query().Get("notice")
		data.Error = r.URL.Query().Get("error")
		if data.LoggedIn && data.Stats == nil {
			log.Info("admin dashboard loaded without stats")
		}
		renderAdmin(w, data)
	}
}

func NewAdminLoginHandler(log *slog.Logger, auth Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin?error="+template.URLQueryEscaper("некорректная форма"), http.StatusSeeOther)
			return
		}

		token, err := auth.Login(strings.TrimSpace(r.FormValue("name")), r.FormValue("password"))
		if err != nil {
			log.Info("web login failed", "name", r.FormValue("name"))
			http.Redirect(w, r, "/admin?error="+template.URLQueryEscaper("неверные учётные данные"), http.StatusSeeOther)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     tokenCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		log.Info("web login successful")
		http.Redirect(w, r, "/admin?notice="+template.URLQueryEscaper("вход выполнен"), http.StatusSeeOther)
	}
}

func NewAdminLogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     tokenCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/admin?notice="+template.URLQueryEscaper("вы вышли из панели"), http.StatusSeeOther)
	}
}

func NewAdminUpdateHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		message := "обновление запущено"
		if err := updater.Update(r.Context()); err != nil {
			if errors.Is(err, core.ErrAlreadyExists) {
				message = "обновление уже выполняется"
			} else {
				log.Error("web update failed", "error", err)
				http.Redirect(w, r, "/admin?error="+template.URLQueryEscaper("ошибка обновления"), http.StatusSeeOther)
				return
			}
		}
		http.Redirect(w, r, "/admin?notice="+template.URLQueryEscaper(message), http.StatusSeeOther)
	}
}

func NewAdminDropHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := updater.Drop(r.Context()); err != nil {
			log.Error("web drop failed", "error", err)
			http.Redirect(w, r, "/admin?error="+template.URLQueryEscaper("ошибка удаления базы"), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin?notice="+template.URLQueryEscaper("база удалена"), http.StatusSeeOther)
	}
}
