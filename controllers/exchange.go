package controllers

import (
  "net/http"

  "golang.org/x/net/context"

  "github.com/gin-gonic/gin"
  "github.com/gin-contrib/sessions"
  oidc "github.com/coreos/go-oidc"

  "golang-idp-fe/config"
  "golang-idp-fe/environment"
  //"golang-idp-fe/gateway/idpbe"
)

func ExchangeAuthorizationCodeCallback(env *environment.State, route environment.Route) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    environment.DebugLog(route.LogId, "exchangeAuthorizationCodeCallback", "", c.MustGet(environment.RequestIdKey).(string))
    session := sessions.Default(c)
    v := session.Get(environment.SessionStateKey)
    if v == nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": "Request not initiated by idp-fe route.LogId. Hint: Missing "+environment.SessionStateKey+" in session"})
      c.Abort()
      return;
    }
    sessionState := v.(string)

    // FIXME: Cleanup the session state once consumed, but where?

    requestState := c.Query("state")
    if requestState == "" {
      c.JSON(http.StatusBadRequest, gin.H{"error": "No state found. Hint: Missing state in query"})
      c.Abort()
      return;
    }

    if requestState != sessionState {
      c.JSON(http.StatusBadRequest, gin.H{"error": "Request did not originate from app. Hint: session state and request state differs"})
      c.Abort()
      return;
    }

    code := c.Query("code")
    if code == "" {
      c.JSON(http.StatusBadRequest, gin.H{"error": "No code to exchange for an access token. Hint: Missing code in query"})
      c.Abort()
      return;
    }

    // Found a code try and exchange it for access token.
    token, err := env.HydraConfig.Exchange(context.Background(), code)
    if err != nil {
      c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
      c.Abort()
      return
    }

    if token.Valid() == true {

      rawIdToken, ok := token.Extra("id_token").(string)
      if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "No id_token found with access token"})
        c.Abort()
        return
      }

      oidcConfig := &oidc.Config{
        ClientID: config.IdpFe.ClientId,
      }
      verifier := env.Provider.Verifier(oidcConfig)

      idToken, err := verifier.Verify(context.Background(), rawIdToken)
      if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify id_token. Hint: " + err.Error()})
        return
      }

      session := sessions.Default(c)
      session.Set(environment.SessionTokenKey, token)
      session.Set(environment.SessionIdTokenKey, idToken)
      err = session.Save()
      if err == nil {
        var redirectTo = config.IdpFe.DefaultRedirectUrl // FIXME: Where to redirect to?
        environment.DebugLog(route.LogId, "exchangeAuthorizationCodeCallback", "Redirecting to: " + redirectTo, c.MustGet(environment.RequestIdKey).(string))
        c.Redirect(http.StatusFound, redirectTo)
        c.Abort()
        return;
      }

      environment.DebugLog(route.LogId, "exchangeAuthorizationCodeCallback", "Failed to save session data: " + err.Error(), c.MustGet("RequestId").(string))
      c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to save session data"})
      c.Abort()
      return
    }

    // Deny by default.
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Exchanged token was invalid. Hint: The timeout on the token might be to short"})
    c.Abort()
    return
  }
  return gin.HandlerFunc(fn)
}