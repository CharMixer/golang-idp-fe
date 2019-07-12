package controllers

import (
  "fmt"
  "strings"
  //"net/url"
  "net/http"

  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"

  "golang-idp-fe/config"
  "golang-idp-fe/environment"
  "golang-idp-fe/gateway/idpbe"
)

type registrationForm struct {
    Username string `form:"username"`
    Name string `form:"display-name"`
    Email string `form:"email"`
    Password string `form:"password"`
    PasswordRetyped string `form:"password_retyped"`
}

func ShowRegistration(env *environment.State, route environment.Route) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    environment.DebugLog(route.LogId, "showRegistration", "", c.MustGet(environment.RequestIdKey).(string))

    session := sessions.Default(c)

    // Retain the values that was submittet, except passwords ?!
    username := session.Get("register.username")
    displayName := session.Get("register.display-name")
    email := session.Get("register.email")

    success := session.Flashes("register.success")

    errors := session.Flashes("register.errors")
    err := session.Save() // Remove flashes read, and save submit fields
    if err != nil {
      environment.DebugLog(route.LogId, "SubmitRegistration", err.Error(), c.MustGet(environment.RequestIdKey).(string))
    }

    var errorUsername string
    var errorPassword string
    var errorPasswordRetyped string
    var errorEmail string
    var errorDisplayName string

    if len(errors) > 0 {
      errorsMap := errors[0].(map[string][]string)
      for k, v := range errorsMap {
        if k == "errorUsername" && len(v) > 0 {
          errorUsername = strings.Join(v, ", ")
        }

        if k == "errorPassword" && len(v) > 0 {
          errorPassword = strings.Join(v, ", ")
        }

        if k == "errorPasswordRetyped" && len(v) > 0 {
          errorPasswordRetyped = strings.Join(v, ", ")
        }

        if k == "errorEmail" && len(v) > 0 {
          errorEmail = strings.Join(v, ", ")
        }

        if k == "errorDisplayName" && len(v) > 0 {
          errorDisplayName = strings.Join(v, ", ")
        }
      }
    }

    c.HTML(200, "register.html", gin.H{
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "username": username,
      "displayName": displayName,
      "email": email,
      "success": success,
      "errorUsername": errorUsername,
      "errorPassword": errorPassword,
      "errorPasswordRetyped": errorPasswordRetyped,
      "errorEmail": errorEmail,
      "errorDisplayName": errorDisplayName,
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitRegistration(env *environment.State, route environment.Route) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    environment.DebugLog(route.LogId, "SubmitRegistration", "", c.MustGet(environment.RequestIdKey).(string))

    var form registrationForm
    err := c.Bind(&form)
    if err != nil {
      // Do better error handling in the application.
      c.JSON(400, gin.H{"error": err.Error()})
      c.Abort()
      return
    }

    session := sessions.Default(c)

    // Save values if submit fails
    session.Set("register.username", form.Username)
    session.Set("register.display-name", form.Name)
    session.Set("register.email", form.Email)
    err = session.Save()
    if err != nil {
      environment.DebugLog(route.LogId, "SubmitRegistration", err.Error(), c.MustGet(environment.RequestIdKey).(string))
    }

    errors := make(map[string][]string)

    username := strings.TrimSpace(form.Username)
    if username == "" {
      errors["errorUsername"] = append(errors["errorUsername"], "Missing username")
    }

    // FIXME: should we trim passwords?
    password := strings.TrimSpace(form.Password)
    if password == "" {
      errors["errorPassword"] = append(errors["errorPassword"], "Missing password")
    }

    retypedPassword := strings.TrimSpace(form.PasswordRetyped)
    if retypedPassword == "" {
      errors["errorPasswordRetyped"] = append(errors["errorPasswordRetyped"], "Missing password")
    }

    if retypedPassword != password {
      errors["errorPasswordRetyped"] = append(errors["errorPasswordRetyped"], "Must match password")
    }

    if len(errors) > 0 {
      session.AddFlash(errors, "register.errors")
      err = session.Save()
      if err != nil {
        environment.DebugLog(route.LogId, "SubmitRegistration", err.Error(), c.MustGet(environment.RequestIdKey).(string))
      }
      c.Redirect(http.StatusFound, route.URL)
      c.Abort();
      return
    }

    if password == retypedPassword { // Just for safety is caught in the input error detection.

      idpbeClient := idpbe.NewIdpBeClient(env.IdpBeConfig)

      var profileRequest = idpbe.Profile{
        Id: form.Username,
        Email: form.Email,
        Password: form.Password,
        Name: form.Name,
      }
      fmt.Println(profileRequest)
      profile, err := idpbe.CreateProfile(config.IdpBe.IdentitiesUrl, idpbeClient, profileRequest)
      if err != nil {
        fmt.Println(err)
        c.JSON(400, gin.H{"error": err.Error()})
        c.Abort()
        return
      }

      session := sessions.Default(c)
      session.Set(environment.SessionSubject, profile.Id)

      // Cleanup session
      session.Delete("register.username")
      session.Delete("register.display-name")
      session.Delete("register.email")

      // Register success message
      session.AddFlash(1, "register.success")

      err = session.Save()
      if err != nil {
        environment.DebugLog(route.LogId, "SubmitRegistration", err.Error(), c.MustGet(environment.RequestIdKey).(string))
      }

      // Registration successful, return to create new ones, but with success message
      c.Redirect(http.StatusFound, "/register")
      c.Abort()
      return
    }

    // Deny by default. Failed to fill in the form correctly.
    c.Redirect(302, route.URL)
    c.Abort()
  }
  return gin.HandlerFunc(fn)
}