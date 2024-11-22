package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

type Router struct {
	db           *sql.DB
	authProvider AuthProvider
	jwtSecret    []byte
	rootURL      *url.URL
	bot          *DiscordBot
	mailchimp    *MailchimpClient
}

const AUTH_COOKIE_NAME string = "csc-auth"

func (r *Router) signin(w http.ResponseWriter, req *http.Request) {
	attributes := r.authProvider.attributesFromContext(req.Context())

	now := time.Now()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"buck_id": attributes.BuckID,
		"iat":     now.Unix(),
		"exp":     now.AddDate(1, 0, 0).Unix(),
	})

	signedTokenString, err := token.SignedString(r.jwtSecret)

	if err != nil {
		log.Fatalln("Failed to sign JWT:", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     AUTH_COOKIE_NAME,
		HttpOnly: true,
		Value:    signedTokenString,
		MaxAge:   365 * 24 * 60 * 60, // 1 year
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	nameNum := strings.TrimSuffix(attributes.Email, "@osu.edu")
	nameNum = strings.TrimSuffix(nameNum, "@buckeyemail.osu.edu")

	student := false
	alum := false
	employee := false
	faculty := false

	for _, affiliation := range attributes.Affiliations {
		if affiliation == "student@osu.edu" {
			student = true
		} else if affiliation == "alum@osu.edu" {
			alum = true
		} else if affiliation == "employee@osu.edu" {
			employee = true
		} else if affiliation == "faculty@osu.edu" {
			faculty = true
		}
	}

	// Upsert the user, updating their information if it already exists. Do not
	// update last_attended_timestamp, otherwise it will be set to NULL
	_, err = r.db.Exec(`
		INSERT INTO users (buck_id, name_num, display_name, student, alum, employee, faculty)
		VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)
		ON CONFLICT (buck_id) DO UPDATE SET
			name_num = name_num,
			display_name = display_name,
			student = student,
			alum = alum,
			employee = employee,
			faculty = faculty
	`, attributes.BuckID, nameNum, attributes.DisplayName, student, alum, employee, faculty)
	if err != nil {
		log.Println("Failed to upsert user:", err)
		http.Error(w, "Failed to sign in. Contact an admin", http.StatusInternalServerError)
		return
	}

	redirect := req.URL.Query().Get("redirect")
	if redirect != "" {
		http.Redirect(w, req, redirect, http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
}

func (r *Router) signout(w http.ResponseWriter, req *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     AUTH_COOKIE_NAME,
		HttpOnly: true,
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})

	r.authProvider.logout(w, req)

	http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
}

func getUserIDFromContext(ctx context.Context) (string, bool) {
	userId, ok := ctx.Value(CONTEXT_USER_ID_KEY).(string)

	return userId, ok
}

func (r *Router) index(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		err := Templates.ExecuteTemplate(w, "404.html.tpl", nil)
		if err != nil {
			log.Println("Failed to render template:", err)
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			return
		}
		return
	}

	userId, hasUserId := getUserIDFromContext(req.Context())

	if hasUserId {
		row := r.db.QueryRow(`
			UPDATE users SET last_seen_timestamp = strftime('%s', 'now') WHERE buck_id = ?1
			RETURNING name_num, discord_id, added_to_mailinglist
		`, userId)
		var nameNum string
		var discordId sql.NullString
		var isOnMailingList bool
		err := row.Scan(&nameNum, &discordId, &isOnMailingList)
		if err != nil {
			log.Println("Failed to get user:", err, userId)
			http.Redirect(w, req, "/signout", http.StatusTemporaryRedirect)
			return
		}

		canAttend, err := getCanAttend(r.db, userId)
		if err != nil {
			log.Println("Failed to get last attendance:", err)
			http.Error(w, "Failed to get last attendance", http.StatusInternalServerError)
			return
		}

		if !isOnMailingList {
			isOnMailingList, err = r.mailchimp.CheckIfMemberOnList(nameNum + "@osu.edu")
			if err != nil {
				log.Println("Failed to check if user is on mailing list:", err)
			}

			if !isOnMailingList {
				isOnMailingList, err = r.mailchimp.CheckIfMemberOnList(nameNum + "@buckeyemail.osu.edu")
				if err != nil {
					log.Println("Failed to check if user is on mailing list:", err)
				}
			}

			if isOnMailingList {
				_, err = r.db.Exec("UPDATE users SET added_to_mailinglist = 1 WHERE buck_id = ?", userId)
				if err != nil {
					log.Println("Failed to update user mailing list status:", err)
					http.Error(w, "Failed to update user mailing list status", http.StatusInternalServerError)
					return
				}
			}
		}

		err = Templates.ExecuteTemplate(w, "index.html.tpl", map[string]interface{}{
			"nameNum":          nameNum,
			"canAttend":        canAttend,
			"hasLinkedDiscord": discordId.Valid,
			"isOnMailingList":  isOnMailingList,
		})
		if err != nil {
			log.Println("Failed to render template:", err)
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			return
		}
	} else {
		err := Templates.ExecuteTemplate(w, "index-unauthed.html.tpl", nil)
		if err != nil {
			log.Println("Failed to render template:", err)
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

func (r *Router) attendance(w http.ResponseWriter, req *http.Request) {
	userId, _ := getUserIDFromContext(req.Context())

	row := r.db.QueryRow("SELECT name_num FROM users WHERE buck_id = ?", userId)
	var nameNum string
	err := row.Scan(&nameNum)
	if err != nil {
		log.Println("Failed to get user:", err, userId)
		http.Redirect(w, req, "/signout", http.StatusTemporaryRedirect)
		return
	}

	rows, err := r.db.Query("SELECT timestamp, kind FROM attendance_records WHERE user_id = ? ORDER BY timestamp DESC", userId)
	if err != nil {
		log.Println("Failed to get attendance records:", err)
		http.Error(w, "Failed to get attendance records", http.StatusInternalServerError)
		return
	}

	ny, _ := time.LoadLocation("America/New_York")

	var attendanceRecords []map[string]interface{}
	for rows.Next() {
		var timestamp int64
		var kind int
		err = rows.Scan(&timestamp, &kind)
		if err != nil {
			log.Println("Failed to scan attendance record:", err)
			http.Error(w, "Failed to scan attendance record", http.StatusInternalServerError)
			return
		}
		attendanceType := "In Person"
		if kind == int(AttendanceTypeOnline) {
			attendanceType = "Online"
		}

		attendanceRecords = append(attendanceRecords, map[string]interface{}{
			"timestamp": time.Unix(timestamp, 0).In(ny).Format("Mon Jan _2, 2006 at 15:04"),
			"type":      attendanceType,
		})
	}

	err = Templates.ExecuteTemplate(w, "attendance.html.tpl", map[string]interface{}{
		"nameNum": nameNum,
		"records": attendanceRecords,
	})
	if err != nil {
		log.Println("Failed to render template:", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

func getLastAttendanceTime(db *sql.DB, userId string) (time.Time, error) {
	row := db.QueryRow("SELECT COALESCE(last_attended_timestamp, 0) FROM users WHERE buck_id = ?", userId)
	var lastAttendanceTimestamp int64
	err := row.Scan(&lastAttendanceTimestamp)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(lastAttendanceTimestamp, 0), nil
}

func getCanAttend(db *sql.DB, userId string) (bool, error) {
	lastAttendanceTime, err := getLastAttendanceTime(db, userId)
	if err != nil {
		return false, err
	}

	return time.Since(lastAttendanceTime) > 24*time.Hour, nil
}

type AttendanceType int

const (
	AttendanceTypeInPerson AttendanceType = iota
	AttendanceTypeOnline
)

func (r *Router) attend(w http.ResponseWriter, req *http.Request) {
	userId, _ := getUserIDFromContext(req.Context())

	attendanceType := AttendanceTypeInPerson
	if req.URL.Path == "/attend/online" {
		attendanceType = AttendanceTypeOnline
	}

	canAttend, err := getCanAttend(r.db, userId)
	if err != nil {
		log.Println("Failed to get last attendance:", err)
		http.Error(w, "Failed to get last attendance", http.StatusInternalServerError)
		return
	}
	if !canAttend {
		log.Println("User attempted to attend too soon")
		http.Error(w, "You cannot attend again so soon", http.StatusForbidden)
		return
	}

	now := time.Now()
	tx, err := r.db.Begin()
	if err != nil {
		log.Println("Attend: Failed to start transaction", err, "User id =", userId)
		http.Error(w, "Failed to get user", http.StatusForbidden)
		return
	}

	_, err = tx.Exec("UPDATE users SET last_attended_timestamp = ?1 WHERE buck_id = ?2", now.Unix(), userId)
	if err != nil {
		log.Println("Attend: Failed to set last attended timestamp:", err)
		http.Error(w, "Failed to set last attended timestamp", http.StatusInternalServerError)
		_ = tx.Rollback()
		return
	}

	_, err = tx.Exec("INSERT INTO attendance_records (user_id, timestamp, kind) VALUES (?1, ?2, ?3)", userId, now.Unix(), attendanceType)
	if err != nil {
		log.Println("Attend: Failed to insert attendance record:", err)
		http.Error(w, "Failed to insert attendance record", http.StatusInternalServerError)
		_ = tx.Rollback()
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Println("Attend: failed to commit transaction:", err)
		http.Error(w, "Failed to attend", http.StatusInternalServerError)
		_ = tx.Rollback()
		return
	}

	err = Templates.ExecuteTemplate(w, "attend-status.html.tpl", map[string]interface{}{
		"canAttend": false,
	})
	if err != nil {
		log.Println("Failed to render template:", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

type contextUserIdType int

const CONTEXT_USER_ID_KEY contextUserIdType = iota

func (r *Router) InjectJwtMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie(AUTH_COOKIE_NAME)
		if err != nil {
			handler.ServeHTTP(w, req)
			return
		}

		token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			return r.jwtSecret, nil
		})
		if err != nil {
			log.Println(err)
			http.Redirect(w, req, fmt.Sprintf("/signin?redirect=%v", req.URL.Path), http.StatusTemporaryRedirect)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			log.Println("Invalid token", token)
			http.Redirect(w, req, fmt.Sprintf("/signin?redirect=%v", req.URL.Path), http.StatusTemporaryRedirect)
			return
		}

		buck_id := claims["buck_id"]

		req = req.WithContext(context.WithValue(req.Context(), CONTEXT_USER_ID_KEY, buck_id))
		handler.ServeHTTP(w, req)
	})
}

func (r *Router) EnforceJwtMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, hasUserId := getUserIDFromContext(req.Context())
		if !hasUserId {
			http.Redirect(w, req, fmt.Sprintf("/signin?redirect=%v", req.URL.Path), http.StatusTemporaryRedirect)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

//go:embed migrations/*
var migrations embed.FS

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Failed to load .env", err)
	}

	mux := http.NewServeMux()

	db, err := sql.Open("sqlite", "./auth.db")
	if err != nil {
		log.Fatalln("Failed to load the database:", err)
	}

	db.SetMaxOpenConns(1)

	_, err = db.Exec("PRAGMA busy_timeout = 5000;")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec("PRAGMA synchronous = NORMAL;")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec("PRAGMA cache_size = 2000;")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec("PRAGMA temp_store = MEMORY;")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec("PRAGMA foreign_keys = TRUE;")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec("PRAGMA locking_mode=IMMEDIATE;")
	if err != nil {
		panic(err)
	}

	dirs, err := migrations.ReadDir("migrations")
	if err != nil {
		log.Fatalln("Failed to read migrations directory:", err)
	}

	slices.SortStableFunc(dirs, func(a fs.DirEntry, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	for _, entry := range dirs {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		data, err := migrations.ReadFile(fmt.Sprintf("migrations/%v", entry.Name()))
		if err != nil {
			log.Fatalln("Failed to read", entry.Name(), err)
		}

		sql := string(data)

		_, err = db.Exec(sql)
		if err != nil {
			log.Fatalln("Failed to run", entry.Name(), err)
		}
	}

	authEnvironment := os.Getenv("ENV")

	// seed the database in dev mode
	if authEnvironment == "" || authEnvironment == "saml" {
		fileName := "migrations/seed.sql"

		data, err := migrations.ReadFile(fileName)
		if err != nil {
			log.Fatalln("Failed to read", fileName, err)
		}

		sql := string(data)

		_, err = db.Exec(sql)
		if err != nil {
			log.Fatalln("Failed to run", fileName, err)
		}
	}

	bot := &DiscordBot{
		Token:         os.Getenv("DISCORD_BOT_TOKEN"),
		GuildId:       os.Getenv("DISCORD_GUILD_ID"),
		AdminRoleId:   os.Getenv("DISCORD_ADMIN_ROLE_ID"),
		StudentRoleId: os.Getenv("DISCORD_STUDENT_ROLE_ID"),
		ClientId:      os.Getenv("DISCORD_CLIENT_ID"),
		ClientSecret:  os.Getenv("DISCORD_CLIENT_SECRET"),
		Db:            db,
	}
	bot.Connect()

	var authProvider AuthProvider
	var rootURL *url.URL

	if authEnvironment == "" {
		rootURL, _ = url.Parse("http://localhost:3000")
		authProvider = mockAuthProvider()
	} else {
		rootURL, err = url.Parse("https://auth.osucyber.club")
		if err != nil {
			panic(err)
		}
		if authEnvironment == "saml" {
			rootURL, err = url.Parse("https://auth-test.osucyber.club")
			if err != nil {
				panic(err)
			}
		}

		keyPair, err := tls.LoadX509KeyPair("keys/sp-cert.pem", "keys/sp-key.pem")
		if err != nil {
			panic(err)
		}

		authProvider, err = samlAuthProvider(mux, rootURL, &keyPair)
		if err != nil {
			panic(err)
		}
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if authEnvironment != "" && authEnvironment != "saml" {
			log.Fatalln("JWT_SECRET not set")
		}

		log.Println("DEFAULTING JWT_SECRET TO `secret` DO NOT RUN IN PRODUCTION")
		jwtSecret = "secret"
	}

	var mailchimp *MailchimpClient
	mailchimpKey := os.Getenv("MAILCHIMP_API_KEY")
	mailchimpServer := os.Getenv("MAILCHIMP_SERVER")
	if mailchimpServer == "" {
		mailchimpServer = "us16"
	}

	// Find in Audience > Settings > Audience name and campaign defaults
	mailchimpListId := os.Getenv("MAILCHIMP_LIST_ID")
	if mailchimpKey == "" || mailchimpServer == "" || mailchimpListId == "" {
		log.Println("Warning: mailchimp key is not configured")
	} else {
		mailchimp = &MailchimpClient{
			ApiKey: mailchimpKey,
			Server: mailchimpServer,
			ListId: mailchimpListId,
		}
	}

	router := &Router{
		db:           db,
		authProvider: authProvider,
		jwtSecret:    []byte(jwtSecret),
		bot:          bot,
		rootURL:      rootURL,
		mailchimp:    mailchimp,
	}

	mux.Handle("/", router.InjectJwtMiddleware(http.HandlerFunc(router.index)))
	mux.Handle("POST /mailchimp", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.SetMailchimp))))
	mux.Handle("/vote", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.vote))))
	mux.Handle("POST /vote", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.processVote))))
	mux.Handle("/admin", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.admin))))
	mux.Handle("/admin/users", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.adminUsers))))
	mux.Handle("/admin/vote", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.adminVote))))
	mux.Handle("/attendance", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.attendance))))
	mux.Handle("/attend/in-person", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.attend))))
	mux.Handle("/attend/online", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.attend))))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/signin", authProvider.requireAuth(http.HandlerFunc(router.signin)))
	mux.Handle("/signout", http.HandlerFunc(router.signout))
	mux.Handle("/discord/signin", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.DiscordSignin))))
	mux.Handle("/discord/callback", router.InjectJwtMiddleware(router.EnforceJwtMiddleware(http.HandlerFunc(router.DiscordCallback))))

	if authEnvironment == "saml" {
		log.Println("Starting server on :443. Visit https://auth-test.osucyber.club and accept the self-signed certificate")
		keyPair, err := getTlsCert()
		if err != nil {
			panic(err)
		}
		server := &http.Server{
			Addr:              ":443",
			ReadHeaderTimeout: time.Second * 10,
			Handler:           mux,
			TLSConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{*keyPair},
			},
		}
		_ = server.ListenAndServeTLS("", "")
	} else {
		if authEnvironment == "" {
			log.Println("Starting server on :3000. Visit http://localhost:3000")
		} else {
			log.Println("Starting server on :3000")
		}

		server := &http.Server{
			Addr:              ":3000",
			ReadHeaderTimeout: time.Second * 10,
			Handler:           mux,
		}
		_ = server.ListenAndServe()
	}
}
