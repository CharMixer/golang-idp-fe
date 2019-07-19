package cpbe

import (
  "net/http"
  "encoding/json"
  "io/ioutil"
  "strings"
  "errors"

  "golang.org/x/net/context"
  "golang.org/x/oauth2/clientcredentials"
)

type ConsentRequest struct {
  Subject string `json:"sub" binding:"required"`
  App string `json:"app" binding:"required"`
  ClientId string `json:"client_id,omitempty"`
  RequestedScopes []string `json:"requested_scopes,omitempty"`
}

type CpBeClient struct {
  *http.Client
}

func NewCpBeClient(config *clientcredentials.Config) *CpBeClient {
  ctx := context.Background()
  client := config.Client(ctx)
  return &CpBeClient{client}
}

func FetchConsents(authorizationsUrl string, client *CpBeClient, consentRequest ConsentRequest) ([]string, error) {

  rawRequest, err := http.NewRequest("GET", authorizationsUrl, nil)
  if err != nil {
    return nil, err
  }

  query := rawRequest.URL.Query()
  query.Add("id", consentRequest.Subject)
  query.Add("app", consentRequest.App)
  if consentRequest.ClientId != "" {
    query.Add("client_id", consentRequest.ClientId)
  }
  if len(consentRequest.RequestedScopes) > 0 {
    query.Add("scope", strings.Join(consentRequest.RequestedScopes, ","))
  }
  rawRequest.URL.RawQuery = query.Encode()

  rawResponse, err := client.Do(rawRequest)
  if err != nil {
    return nil, err
  }

  responseData, err := ioutil.ReadAll(rawResponse.Body)
  if err != nil {
    return nil, err
  }

  if rawResponse.StatusCode != 200 {
    return nil, errors.New("Failed to fetch consents")
  }

  var grantedConsents []string
  err = json.Unmarshal(responseData, &grantedConsents)
  if err != nil {
    return nil, err
  }
  return grantedConsents, nil
}
