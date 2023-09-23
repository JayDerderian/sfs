package server

import (
	"net/http"

	"github.com/sfs/pkg/auth"
)

// add json header to requests. added to middleware stack
// during router instantiation.
func ContentTypeJson(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json;charset=utf8")
		h.ServeHTTP(w, r)
	})
}

func GetAuthenticatedUser(w http.ResponseWriter, r *http.Request) (*auth.User, error) {
	// // TODO: validate the jwt session token in the request
	// userID := jwtstuff...

	// // attempt to find data about the user from the the user db
	// u, err := findUser(userID, getDBConn("Users"))
	// if err != nil {
	// 	ServerErr(w, err.Error())
	// 	return nil, err
	// } else if u == nil {
	// 	NotFound(w, r, fmt.Sprintf("user %s not found", userID))
	// 	return nil, nil
	// }
	// return u, nil
	return nil, nil
}

// get user info
func AuthUserHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := GetAuthenticatedUser(w, r)
		if err != nil {
			ServerErr(w, "failed to get authenticated user")
			return
		}
		h.ServeHTTP(w, r)
	})
}

// ------- admin router --------------------------------

// // A completely separate router for administrator routes
// func adminRouter() http.Handler {
// 	r := chi.NewRouter()
// 	r.Use(AdminOnly)
// 	// TODO: admin handlers
// 	// r.Get("/", adminIndex)
// 	// r.Get("/accounts", adminListAccounts)
// 	return r
// }

// func AdminOnly(h http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		ctx := r.Context()
// 		_, ok := ctx.Value("acl.permission").(float64)
// 		if !ok {
// 			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
// 			return
// 		}
// 		h.ServeHTTP(w, r)
// 	})
// }
