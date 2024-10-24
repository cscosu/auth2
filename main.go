package main

import (
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

	"github.com/k0kubun/pp/v3"
	_ "modernc.org/sqlite"
)

func hello(s AuthProvider) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("/hello")

		attributes := s.attributesFromContext(r.Context())
		pp.Println(attributes)

		fmt.Fprintf(w, "Bye, %s!", attributes.GivenName)
	}
}

//go:embed migrations/*
var migrations embed.FS

func main() {
	mux := http.NewServeMux()

	db, err := sql.Open("sqlite", "./auth.db")

	if err != nil {
		log.Fatalln("Failed to load the database:", err)
	}

	dirs, err := migrations.ReadDir("migrations")

	if err != nil {
		log.Fatalln("Failed to read migrations directory:", err)
	}

	slices.SortStableFunc(dirs, func(a fs.DirEntry, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	for _, entry := range dirs {
		data, err := migrations.ReadFile(fmt.Sprintf("migrations/%v", entry.Name()))
		if err != nil {
			log.Fatalln("Failed to read", entry.Name(), err)
		}
		sql := string(data)
		fmt.Println(sql)
		_, err = db.Exec(sql)
		if err != nil {
			log.Fatalln("Failed to run", entry.Name(), err)
		}
	}

	authEnvironment := os.Getenv("ENV")
	var authProvider AuthProvider

	if authEnvironment == "" {
		authProvider = mockAuthProvider()
	} else {
		rootURL, _ := url.Parse("https://auth.osucyber.club")
		if authEnvironment == "saml" {
			rootURL, _ = url.Parse("https://auth-test.osucyber.club")
		}

		keyPair, err := tls.LoadX509KeyPair("keys/sp-cert.pem", "keys/sp-key.pem")
		if err != nil {
			panic(err)
		}

		authProvider, _ = samlAuthProvider(mux, rootURL, &keyPair)
	}

	mux.Handle("/hello", authProvider.requireAuth(http.HandlerFunc(hello(authProvider))))
	mux.Handle("/logout", authProvider.requireAuth(http.HandlerFunc(authProvider.globalLogout)))

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
