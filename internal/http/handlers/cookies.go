package handlers

import "net/http"

var cookieSecure bool
var cookieSameSite = http.SameSiteLaxMode

func SetCookieConfig(secure bool, sameSite http.SameSite) {
	cookieSecure = secure
	cookieSameSite = sameSite
}
