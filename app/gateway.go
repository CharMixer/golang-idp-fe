package app

import (
  "net/url"
  "net/http"
  "crypto/rand"
  "encoding/base64"
  "golang.org/x/oauth2"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gin-contrib/sessions"
  oidc "github.com/coreos/go-oidc"

  idp "github.com/charmixer/idp/client"
  "github.com/charmixer/idpui/config"
  "github.com/charmixer/idpui/environment"
)

// Use this handler as middleware to enable gateway functions in controllers
func LoadIdentity(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    var idToken *oidc.IDToken

    session := sessions.Default(c)
    t := session.Get(environment.SessionIdTokenKey)
    if t == nil {
      c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing id_token in session"})
      return
    }

    idToken = t.(*oidc.IDToken)
    if idToken == nil {
      c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing id_token in session"})
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    idpClient := idp.NewIdpClientWithUserAccessToken(env.HydraConfig, accessToken)

    // Look up profile information for user.
    identityRequest := []idp.ReadHumansRequest{ {Id: idToken.Subject} }
    _, humans, err := idp.ReadHumans(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.identities"), identityRequest)
    if err != nil {
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }
    if humans == nil {
      c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Identity not found"})
      return
    }

    status, obj, restError := idp.UnmarshalResponse(0, humans)
    if status == 200 && obj != nil {
      human := obj.(idp.Human)
      c.Set("identity", human)
      c.Next()
    }

    // Deny by default
    logrus.Debug(restError)
    logrus.WithFields(logrus.Fields{ "status":status }).Debug("Unmarshal response failed")
    c.AbortWithStatus(http.StatusForbidden)
  }
  return gin.HandlerFunc(fn)
}

func RequireIdentity(c *gin.Context) *idp.Human {
  identity, exists := c.Get("identity")
  if exists == true {
    return identity.(*idp.Human)
  }
  return nil
}

func IdpClientUsingAuthorizationCode(env *environment.State, c *gin.Context) (*idp.IdpClient) {
  session := sessions.Default(c)
  t := session.Get(environment.SessionTokenKey)
  if t != nil {
    accessToken := t.(*oauth2.Token)
    return idp.NewIdpClientWithUserAccessToken(env.HydraConfig, accessToken)
  }
  return nil
}

func IdpClientUsingClientCredentials(env *environment.State, c *gin.Context) (*idp.IdpClient) {
  return idp.NewIdpClient(env.IdpApiConfig)
}

func CreateRandomStringWithNumberOfBytes(numberOfBytes int) (string, error) {
  st := make([]byte, numberOfBytes)
  _, err := rand.Read(st)
  if err != nil {
    return "", err
  }
  return base64.StdEncoding.EncodeToString(st), nil
}

func StartAuthenticationSession(env *environment.State, c *gin.Context, log *logrus.Entry) (*url.URL, error) {
  var state string
  var err error

  log = log.WithFields(logrus.Fields{
    "func": "StartAuthentication",
  })

  // Redirect to after successful authentication
  redirectTo := c.Request.RequestURI

  // Always generate a new authentication session state
  session := sessions.Default(c)

  // Create random bytes that are based64 encoded to prevent character problems with the session store.
  // The base 64 means that more than 64 bytes are stored! Which can cause "securecookie: the value is too long"
  // To prevent this we need to use a filesystem store instead of broser cookies.
  state, err = CreateRandomStringWithNumberOfBytes(32);
  if err != nil {
    log.Debug(err.Error())
    return nil, err
  }

  log.Debug(state)
  log.Debug(redirectTo)

  session.Set(environment.SessionStateKey, state)
  session.Set(state, redirectTo)
  err = session.Save()
  if err != nil {
    log.Debug(err.Error())
    return nil, err
  }

  logSession := log.WithFields(logrus.Fields{
    "redirect_to": redirectTo,
    "state": state,
  })
  logSession.Debug("Started session")
  authUrl := env.HydraConfig.AuthCodeURL(state)
  u, err := url.Parse(authUrl)
  return u, err
}