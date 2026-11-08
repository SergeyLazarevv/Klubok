package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// VK — VK ID (OAuth 2.1, без SDK: обычный редирект). Приложение регистрируется на id.vk.com (тип «Web»),
// доверенный Redirect URL — <KLUBOK_PUBLIC_URL>/api/v1/auth/vk/callback. Секрет не нужен: клиент публичный,
// код защищает PKCE. Через VK ID входят и аккаунты Одноклассников и Mail.ru.
type VK struct {
	clientID string
	http     *http.Client
}

func NewVK(clientID string) *VK {
	return &VK{clientID: clientID, http: &http.Client{Timeout: 10 * time.Second}}
}

func (v *VK) Name() string { return "vk" }

func (v *VK) AuthURL(state, challenge, redirectURI string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {v.clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"email"},
	}
	return "https://id.vk.com/authorize?" + q.Encode()
}

func (v *VK) Exchange(ctx context.Context, code, verifier, redirectURI string, callback url.Values) (Profile, error) {
	deviceID := callback.Get("device_id")
	if deviceID == "" {
		return Profile{}, errors.New("vk: no device_id in callback")
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	err := postForm(ctx, v.http, "https://id.vk.com/oauth2/auth", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"client_id":     {v.clientID},
		"device_id":     {deviceID},
		"redirect_uri":  {redirectURI},
		"state":         {callback.Get("state")},
	}, &tok)
	if err != nil {
		return Profile{}, err
	}
	if tok.AccessToken == "" {
		return Profile{}, errors.New("vk: empty access token")
	}

	var info struct {
		User struct {
			UserID    flexID `json:"user_id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Avatar    string `json:"avatar"`
			Email     string `json:"email"`
		} `json:"user"`
	}
	err = postForm(ctx, v.http, "https://id.vk.com/oauth2/user_info", url.Values{
		"client_id":    {v.clientID},
		"access_token": {tok.AccessToken},
	}, &info)
	if err != nil {
		return Profile{}, err
	}
	u := info.User
	if u.UserID == "" {
		return Profile{}, errors.New("vk: empty user id")
	}
	name := fullName(u.FirstName, u.LastName)
	if name == "" {
		name = "Пользователь VK"
	}
	return Profile{Subject: string(u.UserID), Name: name, Email: u.Email, AvatarURL: u.Avatar}, nil
}

// flexID — id пользователя VK: приходит строкой, но на всякий случай принимаем и число.
type flexID string

func (j *flexID) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		s, err := strconv.Unquote(string(b))
		*j = flexID(s)
		return err
	}
	*j = flexID(b)
	return nil
}
