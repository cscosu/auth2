package main

import (
	"database/sql"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

func (r *Router) EnforceAdminMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userId, _ := getUserIDFromContext(req.Context())

		row := r.db.QueryRow("SELECT is_admin FROM users WHERE buck_id = ?", userId)
		var isAdmin bool
		err := row.Scan(&isAdmin)
		if err != nil {
			log.Println("Failed to get if user is admin:", err, userId)
			http.Redirect(w, req, "/signout", http.StatusTemporaryRedirect)
			return
		}

		if !isAdmin {
			log.Println("User is not admin:", userId)
			http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

func (r *Router) admin(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "/admin/users", http.StatusTemporaryRedirect)
}

type AdminUserList struct {
	DiscordID          int64
	BuckID             string
	DisplayName        string
	NameNum            string
	LastSeenTime       string
	LastAttendedTime   string
	AddedToMailingList bool
	Student            bool
	Alum               bool
	Employee           bool
	Faculty            bool
}

type AdminOrderState struct {
	OrderNum  int
	NextUrl   string
	AddUrl    string
	DeleteUrl string
	IsAsc     bool
	IsDesc    bool
}

func (r *Router) adminUsers(w http.ResponseWriter, req *http.Request) {
	userId, _ := getUserIDFromContext(req.Context())

	row := r.db.QueryRow("SELECT name_num FROM users WHERE buck_id = ?", userId)
	var nameNum string
	err := row.Scan(&nameNum)
	if err != nil {
		log.Println("Failed to get user:", err, userId)
		http.Redirect(w, req, "/signout", http.StatusTemporaryRedirect)
		return
	}

	page := req.URL.Query().Get("page")
	pageNum, _ := strconv.Atoi(page)
	if pageNum == 0 {
		pageNum = 1
	}
	offset := (pageNum - 1) * 50

	searchQuery := req.URL.Query().Get("search")
	orderQuery := req.URL.Query().Get("order")

	// if we are searching, we need to update the url to reflect the search
	if req.Header.Get("HX-Request") == "true" && req.Header.Get("HX-Boosted") == "" {
		query := req.URL.Query()
		if query.Get("search") == "" {
			query.Del("search")
		}
		req.URL.RawQuery = strings.Replace(query.Encode(), "%2C", ",", 1)
		w.Header().Set("HX-Replace-Url", req.URL.String())
	}

	sqlQueryBody := `
		FROM users
		WHERE
			name_num LIKE '%' || ?1 || '%' ESCAPE '\'
			OR discord_id LIKE '%' || ?1 || '%' ESCAPE '\'
			OR buck_id LIKE '%' || ?1 || '%' ESCAPE '\'
			OR display_name LIKE '%' || ?1 || '%' ESCAPE '\'
		ORDER BY
	`

	orders := make(map[string]AdminOrderState)
	tableNames := []string{
		"name_num",
		"discord_id",
		"display_name",
		"last_seen_timestamp",
		"last_attended_timestamp",
		"added_to_mailinglist",
		"student",
		"alum",
		"employee",
		"faculty",
	}

	query := req.URL.Query()
	for _, tableName := range tableNames {
		query.Set("order", strings.TrimRight(tableName+","+orderQuery, ","))
		req.URL.RawQuery = strings.ReplaceAll(query.Encode(), "%2C", ",")
		addUrl := req.URL.String()

		removedOrderQuery := strings.Replace(orderQuery, "-"+tableName, "", 1)
		removedOrderQuery = strings.Replace(removedOrderQuery, tableName, "", 1)
		removedOrderQuery = strings.Replace(removedOrderQuery, ",,", ",", 1)
		removedOrderQuery = strings.Trim(removedOrderQuery, ",")
		if removedOrderQuery != "" {
			query.Set("order", removedOrderQuery)
		} else {
			query.Del("order")
		}
		req.URL.RawQuery = strings.ReplaceAll(query.Encode(), "%2C", ",")
		deleteUrl := req.URL.String()

		orders[tableName] = AdminOrderState{
			AddUrl:    addUrl,
			DeleteUrl: deleteUrl,
		}
	}

	sqlOrders := make([]string, 0)
	orderNum := 1
	for _, tableName := range strings.Split(orderQuery, ",") {
		if len(tableName) > 1 {
			negative := tableName[0] == '-'
			tableName = strings.TrimPrefix(tableName, "-")
			if slices.Contains(tableNames, tableName) {
				order := orders[tableName]
				order.OrderNum = orderNum

				sqlDirection := "DESC"
				if negative {
					order.IsAsc = true
					sqlDirection = "ASC"
				} else {
					order.IsDesc = true
				}

				query := req.URL.Query()
				if order.IsAsc {
					query.Set("order", strings.Replace(orderQuery, "-"+tableName, tableName, 1))
				} else {
					query.Set("order", strings.Replace(orderQuery, tableName, "-"+tableName, 1))
				}
				req.URL.RawQuery = strings.ReplaceAll(query.Encode(), "%2C", ",")
				order.NextUrl = req.URL.String()

				sqlOrders = append(sqlOrders, tableName+" "+sqlDirection)
				orders[tableName] = order
				orderNum++
			}
		}
	}

	if len(sqlOrders) == 0 {
		sqlOrders = append(sqlOrders, "last_seen_timestamp DESC")
	}

	sqlQueryBody += strings.Join(sqlOrders, ", ") + "\n"

	var totalUsers int
	err = r.db.QueryRow("SELECT COUNT(*)"+sqlQueryBody, searchQuery, offset).Scan(&totalUsers)
	if err != nil {
		log.Println("Failed to get total users:", err)
		http.Error(w, "Failed to get users", http.StatusInternalServerError)
		return
	}
	totalPages := ceilDiv(totalUsers, 50)
	pageNumbers := make([]int, totalPages)
	for i := range pageNumbers {
		pageNumbers[i] = i + 1
	}

	users := []AdminUserList{}
	rows, err := r.db.Query(`
		SELECT buck_id, discord_id, name_num, display_name, last_seen_timestamp, last_attended_timestamp, added_to_mailinglist, student, alum, employee, faculty
		`+sqlQueryBody+`
		LIMIT 50
		OFFSET ?2
	`, searchQuery, offset)
	if err != nil {
		log.Println("Failed to get users:", err)
		http.Error(w, "Failed to get users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	ny, _ := time.LoadLocation("America/New_York")

	for rows.Next() {
		var buckID, nameNum, displayName string
		var discordID, lastAttendedTimestamp sql.NullInt64
		var lastSeenTimestamp int64
		var addedToMailingList, student, alum, employee, faculty bool
		err := rows.Scan(&buckID, &discordID, &nameNum, &displayName, &lastSeenTimestamp, &lastAttendedTimestamp, &addedToMailingList, &student, &alum, &employee, &faculty)
		if err != nil {
			log.Println("Failed to scan user:", err)
			http.Error(w, "Failed to get users", http.StatusInternalServerError)
			return
		}

		var lastAttendedTime string
		if lastAttendedTimestamp.Valid {
			lastAttendedTime = time.Unix(lastAttendedTimestamp.Int64, 0).In(ny).Format("Mon Jan _2, 2006 at 15:04")
		}

		users = append(users, AdminUserList{
			DiscordID:          discordID.Int64,
			BuckID:             buckID,
			DisplayName:        displayName,
			NameNum:            nameNum,
			LastSeenTime:       time.Unix(lastSeenTimestamp, 0).In(ny).Format("Mon Jan _2, 2006 at 15:04"),
			LastAttendedTime:   lastAttendedTime,
			AddedToMailingList: addedToMailingList,
			Student:            student,
			Alum:               alum,
			Employee:           employee,
			Faculty:            faculty,
		})
	}

	err = Templates.ExecuteTemplate(w, "admin-users.html.tpl", map[string]interface{}{
		"nameNum":     nameNum,
		"path":        req.URL.Path,
		"users":       users,
		"pageNumbers": pageNumbers,
		"currentPage": pageNum,
		"orders":      orders,
		"orderQuery":  orderQuery,
		"searchQuery": searchQuery,
	})
	if err != nil {
		log.Println("Failed to render template:", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}
