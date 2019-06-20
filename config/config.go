package config

import (
  "os"
  "strings"
)

/*
RedirectURL:  redirect url,
ClientID:     "GOOGLE_CLIENT_ID",
ClientSecret: "CLIENT_SECRET",
Scopes:       []string{"scope1", "scope2"},
Endpoint:     oauth2 endpoint,
*/

type HydraConfig struct {
  Url             string
  AdminUrl        string
}

type OAuth2ClientConfig struct {
  ClientId        string
  ClientSecret    string
  Scopes          []string
  RedirectURL     string
  Endpoint        string
}

type IdpFeConfig struct {
  CsrfAuthKey string
  IdpBackendUrl string
}

var Hydra HydraConfig
var OAuth2Client OAuth2ClientConfig
var IdpFe IdpFeConfig

func InitConfigurations() {
  Hydra.Url                   = getEnvStrict("HYDRA_URL")
  Hydra.AdminUrl              = getEnvStrict("HYDRA_ADMIN_URL")

  OAuth2Client.ClientId       = getEnv("OAUTH2_CLIENT_CLIENT_ID")
  OAuth2Client.ClientSecret   = getEnv("OAUTH2_CLIENT_ClIENT_SECRET")
  OAuth2Client.Scopes         = strings.Split(getEnv("OAUTH2_CLIENT_SCOPES"), ",")
  OAuth2Client.RedirectURL    = getEnv("OAUTH2_CLIENT_REDIRECT_URL")
  OAuth2Client.Endpoint       = getEnv("OAUTH2_CLIENT_ENDPOINT")

  IdpFe.CsrfAuthKey           = getEnv("CSRF_AUTH_KEY") // 32 byte long auth key. When you change this user session will break.
  IdpFe.IdpBackendUrl         = getEnv("IDP_BACKEND_URL")
}

func getEnv(name string) string {
  return os.Getenv(name)
}

func getEnvStrict(name string) string {
  r := getEnv(name)

  if r == "" {
    panic("Missing environment variable: " + name)
  }

  return r
}
