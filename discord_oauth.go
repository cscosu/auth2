package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

const OAUTH_STATE_COOKIE = "oauthstate"

func (r *Router) DiscordSignin(w http.ResponseWriter, req *http.Request) {
	state := generateStateOauthCookie(w)
	redirectUri := fmt.Sprintf("%s/discord/callback", r.rootURL)
	url := fmt.Sprintf("https://discord.com/oauth2/authorize?client_id=%s&response_type=code&redirect_uri=%v&scope=identify+guilds.join&state=%v", r.bot.ClientId, url.QueryEscape(redirectUri), state)
	http.Redirect(w, req, url, http.StatusTemporaryRedirect)
}

func (r *Router) DiscordCallback(w http.ResponseWriter, req *http.Request) {
	userId, _ := getUserIDFromContext(req.Context())

	stateCookie, err := req.Cookie(OAUTH_STATE_COOKIE)
	if err != nil {
		log.Println("Discord callback: Missing oauth state cookie. User id =", userId)
		http.Error(w, "Missing oauthstate cookie", http.StatusBadRequest)
		return
	}
	stateParam := req.URL.Query().Get("state")
	if stateParam == "" {
		log.Println("Discord callback: Missing state url parameter. User id =", userId)
		http.Error(w, "Missing state url parameter", http.StatusBadRequest)
		return
	}
	if stateCookie.Value != stateParam {
		log.Println("Discord callback: State cookie and state parameter don't match. State cookie =", stateCookie.Value, ", state param =", stateParam, "User id =", userId)
		http.Error(w, "State cookie and state parameter don't match", http.StatusBadRequest)
		return
	}

	code := req.URL.Query().Get("code")
	authToken, err := getDiscordAuthToken(r.rootURL, r.bot, code)
	if err != nil {
		log.Println("Discord callback: Error getting discord auth token:", err, "User id =", userId)
		http.Error(w, "Error getting discord auth token", http.StatusForbidden)
		return
	}

	discordUser, err := getDiscordUser(authToken)
	if err != nil {
		log.Println("Discord callback: Error getting discord user:", err, "User id =", userId)
		http.Error(w, "Error getting user information", http.StatusForbidden)
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		log.Println("Discord callback: Failed to start transaction", err, "User id =", userId)
		http.Error(w, "Failed to get user", http.StatusForbidden)
		return
	}
	row := tx.QueryRow("SELECT discord_id FROM users WHERE buck_id = ?", userId)

	var oldDiscordId sql.NullString
	err = row.Scan(&oldDiscordId)
	if err != nil {
		log.Println("Discord callback: failed to get old discord id:", err)
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		_ = tx.Rollback()
		return
	}

	_, err = tx.Exec("UPDATE users SET discord_id = ? WHERE buck_id = ?", discordUser.ID, userId)
	if err != nil {
		log.Println("Discord callback: failed to update user:", err)
		http.Error(w, "Failed to update discord", http.StatusInternalServerError)
		_ = tx.Rollback()
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Println("Discord callback: failed to commit transcation:", err)
		http.Error(w, "Failed to update discord", http.StatusInternalServerError)
		_ = tx.Rollback()
		return
	}

	if oldDiscordId.Valid {
		_ = r.bot.RemoveStudentRole(oldDiscordId.String)
	}
	_ = r.bot.AddStudentToGuild(discordUser.ID, authToken)
	_ = r.bot.GiveStudentRole(discordUser.ID)

	http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
}

func generateStateOauthCookie(w http.ResponseWriter) string {
	var expiration = time.Now().Add(2 * time.Hour)
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	cookie := http.Cookie{Name: OAUTH_STATE_COOKIE, Value: state, Expires: expiration}
	http.SetCookie(w, &cookie)
	return state
}

func getDiscordAuthToken(rootURL *url.URL, b *DiscordBot, code string) (string, error) {
	redirectUri := fmt.Sprintf("%s/discord/callback", rootURL)

	data := url.Values{
		"client_id":     {b.ClientId},
		"client_secret": {b.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectUri},
	}

	req, err := http.NewRequest("POST", "https://discord.com/api/oauth2/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discord token endpoint responded with %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("discord token endpoint response did not have access_token")
	}

	return accessToken, nil
}

type DiscordUser struct {
	Avatar        string `json:"avatar"`
	Discriminator string `json:"discriminator"`
	Email         string `json:"email"`
	Flags         int    `json:"flags"`
	ID            string `json:"id"`
	Username      string `json:"username"`
}

func getDiscordUser(authToken string) (DiscordUser, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	resp, err := client.Do(req)
	if err != nil {
		return DiscordUser{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DiscordUser{}, fmt.Errorf("failed to get user info")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DiscordUser{}, err
	}

	var user DiscordUser
	if err := json.Unmarshal(body, &user); err != nil {
		return DiscordUser{}, err
	}

	return user, nil
}
