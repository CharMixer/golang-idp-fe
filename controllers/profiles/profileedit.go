package profiles

import (
  "strings"
  "net/http"
  "reflect"
  "gopkg.in/go-playground/validator.v9"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  idp "github.com/charmixer/idp/client"

  "github.com/charmixer/idpui/app"
  "github.com/charmixer/idpui/config"
  "github.com/charmixer/idpui/environment"
  "github.com/charmixer/idpui/utils"
  "github.com/charmixer/idpui/validators"
)

type profileEditForm struct {
  Name string `form:"display-name" validate:"required,notblank"`
  Email string `form:"email" validate:"required,email"`
}

func ShowProfileEdit(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowProfile",
    })

    identity := app.RequireIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    // Retain the values that was submittet
    submittetName := session.Get("profileedit.display-name")
    submittetEmail := session.Get("profileedit.email")

    errors := session.Flashes("profileedit.errors")
    err := session.Save() // Remove flashes read, and save submit fields
    if err != nil {
      log.Debug(err.Error())
    }

    // Use submittet value from flash or default from db.
    var displayName string
    if submittetName == nil {
      displayName = identity.Name
    } else {
      displayName = submittetName.(string)
    }

    var email string
    if submittetEmail == nil {
      email = identity.Email
    } else {
      email = submittetEmail.(string)
    }

    var errorEmail string
    var errorDisplayName string

    if len(errors) > 0 {
      errorsMap := errors[0].(map[string][]string)
      for k, v := range errorsMap {

        if k == "email" && len(v) > 0 {
          errorEmail = strings.Join(v, ", ")
        }

        if k == "display-name" && len(v) > 0 {
          errorDisplayName = strings.Join(v, ", ")
        }
      }
    }

    c.HTML(http.StatusOK, "profileedit.html", gin.H{
      "title": "Profile",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "profileEditUrl": "/me/edit",
      "user": identity.Id,
      "displayName": displayName,
      "email": email,
      "errorEmail": errorEmail,
      "errorDisplayName": errorDisplayName,
      "name": identity.Name,
      "registeredDisplayName": identity.Name,
      "registeredEmail": identity.Email,
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitProfileEdit(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitProfileEdit",
    })

    var form profileEditForm
    err := c.Bind(&form)
    if err != nil {
      // Do better error handling in the application.
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      c.Abort()
      return
    }

    identity := app.RequireIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    // Save values if submit fails
    session.Set("profileedit.display-name", form.Name)
    session.Set("profileedit.email", form.Email)
    err = session.Save()
    if err != nil {
      log.Debug(err.Error())
    }

    errors := make(map[string][]string)
    validate := validator.New()
    validate.RegisterValidation("notblank", validators.NotBlank)
    err = validate.Struct(form)
    if err != nil {

      // Validation syntax is invalid
      if err,ok := err.(*validator.InvalidValidationError); ok{
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }

      reflected := reflect.ValueOf(form) // Use reflector to reverse engineer struct
      for _, err := range err.(validator.ValidationErrors){

        // Attempt to find field by name and get json tag name
        field,_ := reflected.Type().FieldByName(err.StructField())
        var name string

        // If form tag doesn't exist, use lower case of name
        if name = field.Tag.Get("form"); name == ""{
          name = strings.ToLower(err.StructField())
        }

        switch err.Tag() {
        case "required":
            errors[name] = append(errors[name], "Field is required")
            break
        case "email":
            errors[name] = append(errors[name], "Field must be a valid email")
            break
        case "eqfield":
            errors[name] = append(errors[name], "Field should be equal to the "+err.Param())
            break
        case "notblank":
          errors[name] = append(errors[name], "Field is not allowed to be blank")
          break
        default:
            errors[name] = append(errors[name], "Field is invalid")
            break
        }
      }

    }

    if len(errors) > 0 {
      session.AddFlash(errors, "profileedit.errors")
      err = session.Save()
      if err != nil {
        log.Debug(err.Error())
      }

      submitUrl, err := utils.FetchSubmitUrlFromRequest(c.Request, nil)
      if err != nil {
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }
      log.WithFields(logrus.Fields{"redirect_to": submitUrl}).Debug("Redirecting")
      c.Redirect(http.StatusFound, submitUrl)
      c.Abort()
      return
    }

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    identityRequest := []idp.UpdateHumansRequest{{
      Id: identity.Id,
      Email: form.Email,
      Name: form.Name,
    }}
    _, updatedHumans, err := idp.UpdateHumans(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.humans.collection"), identityRequest)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if updatedHumans == nil {
      log.Debug("Update failed. Hint: Failed to execute UpdateHumansRequest")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    status, obj, _ := idp.UnmarshalResponse(0, updatedHumans)
    if status == 200 && obj != nil {

      updatedHuman := obj.(idp.Human)

      // Cleanup session
      session.Delete("profileedit.display-name")
      session.Delete("profileedit.email")
      err = session.Save()
      if err != nil {
        log.Debug(err.Error())
      }

      if updatedHuman != (idp.Human{}) {
        log.WithFields(logrus.Fields{"id": updatedHuman.Id}).Debug("Human updated")
        redirectTo := "/"
        log.WithFields(logrus.Fields{"redirect_to": redirectTo}).Debug("Redirecting")
        c.Redirect(http.StatusFound, redirectTo)
        c.Abort()
        return
      }
    }

    // Deny by default. Failed to fill in the form correctly.
    submitUrl, err := utils.FetchSubmitUrlFromRequest(c.Request, nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }
    log.WithFields(logrus.Fields{"redirect_to": submitUrl}).Debug("Redirecting")
    c.Redirect(http.StatusFound, submitUrl)
    c.Abort()
  }
  return gin.HandlerFunc(fn)
}
