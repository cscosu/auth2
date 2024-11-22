package main

import (
	"log"
	"net/http"

	_ "modernc.org/sqlite"
)

var candidates map[string]int
var candidateTitle string

func init() {
	candidates = make(map[string]int)
}

func (r *Router) processVote(w http.ResponseWriter, req *http.Request) {
	userId, hasUserId := getUserIDFromContext(req.Context())

	if hasUserId {
		row := r.db.QueryRow(`
			UPDATE users SET last_seen_timestamp = strftime('%s', 'now') WHERE buck_id = ?1
			RETURNING name_num
		`, userId)
		var nameNum string
		err := row.Scan(&nameNum)
		if err != nil {
			log.Println("Failed to get user:", err, userId)
			http.Redirect(w, req, "/signout", http.StatusTemporaryRedirect)
			return
		}

		// TODO - handle logic for actually processing vote

		err = Templates.ExecuteTemplate(w, "voting-form.html.tpl", map[string]interface{}{
			"nameNum":  nameNum,
			"hasVoted": true,
		})
		if err != nil {
			log.Println("Failed to render template:", err)
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}

}

func (r *Router) vote(w http.ResponseWriter, req *http.Request) {

	userId, hasUserId := getUserIDFromContext(req.Context())

	if hasUserId {
		row := r.db.QueryRow(`
			UPDATE users SET last_seen_timestamp = strftime('%s', 'now') WHERE buck_id = ?1
			RETURNING name_num
		`, userId)
		var nameNum string
		err := row.Scan(&nameNum)
		if err != nil {
			log.Println("Failed to get user:", err, userId)
			http.Redirect(w, req, "/signout", http.StatusTemporaryRedirect)
			return
		}

		var candidateList []string
		candidateList = append(candidateList, "A")
		candidateList = append(candidateList, "B")
		candidateList = append(candidateList, "C")

		err = Templates.ExecuteTemplate(w, "vote.html.tpl", map[string]interface{}{
			"nameNum":        nameNum,
			"hasVoted":       false,         // TODO - get a real value
			"isMember":       true,          // TODO - get a real value
			"candidateTitle": "President",   // TODO - get a real value
			"candidateList":  candidateList, // TODO - get a real value
		})
		if err != nil {
			log.Println("Failed to render template:", err)
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

func (r *Router) adminVote(w http.ResponseWriter, req *http.Request) {
	userId, _ := getUserIDFromContext(req.Context())

	row := r.db.QueryRow("SELECT name_num FROM users WHERE buck_id = ?", userId)
	var nameNum string
	err := row.Scan(&nameNum)
	if err != nil {
		log.Println("Failed to get user:", err, userId)
		http.Redirect(w, req, "/signout", http.StatusTemporaryRedirect)
		return
	}

	var candidateList []string
	for candidate := range candidates {
		candidateList = append(candidateList, candidate)
	}

	err = Templates.ExecuteTemplate(w, "admin-vote.html.tpl", map[string]interface{}{
		"nameNum":        nameNum,
		"path":           req.URL.Path,
		"candidateTitle": candidateTitle,
		"candidateList":  candidateList,
	})
	if err != nil {
		log.Println("Failed to render template:", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}
