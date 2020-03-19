package zoombot

type Credentials struct {
	ClientID          string `json:"client_id"`
	ClientSecret      string `json:"client_secret"`
	VerificationToken string `json:"verification_token"`
}
