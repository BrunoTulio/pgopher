package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/BrunoTulio/logr"
	"github.com/BrunoTulio/pgopher/internal/utils"
	"golang.org/x/oauth2"
)

type Auth struct {
	log logr.Logger
}

func New(log logr.Logger) *Auth {
	return &Auth{log}
}

type OauthInfo struct {
	ClientID         string
	ClientSecret     string
	Scopes           []string
	EndpointAuthURL  string
	EndpointTokenURL string
	RedirectURL      string
}

var (
	configOauthMap = map[string]OauthInfo{
		"drive": {
			ClientID:     os.Getenv("DRIVE_CLIENT_ID"),
			ClientSecret: utils.MustReveal(os.Getenv("DRIVE_CLIENT_SECRET")),
			Scopes: []string{
				"https://www.googleapis.com/auth/drive.file",    // ← OBRIGATÓRIO!
				"https://www.googleapis.com/auth/drive.appdata", // ← OBRIGATÓRIO!
			},
			EndpointAuthURL:  "https://accounts.google.com/o/oauth2/auth",
			EndpointTokenURL: "https://oauth2.googleapis.com/token",
			RedirectURL:      "http://localhost:53682/",
		},
		"dropbox": {
			ClientID:         os.Getenv("DROPBOX_CLIENT_ID"),
			ClientSecret:     utils.MustReveal(os.Getenv("DROPBOX_CLIENT_SECRET")),
			Scopes:           []string{},
			EndpointAuthURL:  "https://www.dropbox.com/oauth2/authorize",
			EndpointTokenURL: "https://api.dropboxapi.com/oauth2/token",
			RedirectURL:      "http://localhost:53682/",
		},
	}
)

func (a *Auth) Run(providerName string) (string, error) {
	providers := []string{"drive", "dropbox"}

	if !slices.Contains(providers, providerName) {
		return "", fmt.Errorf("provider %s does not exist for method auth", providerName)
	}

	info := configOauthMap[providerName]
	conf := &oauth2.Config{
		ClientID:     info.ClientID,
		ClientSecret: info.ClientSecret,
		Scopes:       info.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  info.EndpointAuthURL,
			TokenURL: info.EndpointTokenURL,
		},
		RedirectURL: info.RedirectURL,
	}

	state := randomState()

	authURL := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)

	a.log.Infof("🚀 %s OAuth2 - Autorizando...\n", providerName)
	a.log.Infof("\n📖 Abra este link no navegador:")
	fmt.Println(authURL)
	a.log.Infof("\n⏳ Aguardando callback em http://localhost:53682/...")

	codeChan := make(chan string)
	errChan := make(chan error)

	server := &http.Server{Addr: ":53682"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errChan <- fmt.Errorf("state inválido - possível CSRF attack")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("código de autorização não recebido")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `
            <!DOCTYPE html>
<html lang="pt-BR">
  <head>
    <meta charset="UTF-8" />
    <title>Autenticação concluída</title>
  </head>
  <body style="font-family:sans-serif;text-align:center;padding:50px">
    <h1>✅ Autenticação concluída!</h1>
    <p>Pode fechar esta aba e voltar pro terminal.</p>
  </body>
</html>
        `)

		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:

		return "", fmt.Errorf("❌ Erro: %v\n", err)
	case <-time.After(5 * time.Minute):

		return "", fmt.Errorf("❌ Timeout - processo cancelado após 5min")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)

	a.log.Infof("\n🔄 Trocando código por access token...")
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		return "", fmt.Errorf("❌ Erro ao trocar código: %v\n", err)

	}

	jsonData, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("falha ao serializar token: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(jsonData)
	return encoded, nil
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
